package stacker

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/klauspost/pgzip"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/layer"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type Apply struct {
	layers  []ispec.Descriptor
	opts    BaseLayerOpts
	baseOCI *umoci.Layout
}

func NewApply(opts BaseLayerOpts) (*Apply, error) {
	a := &Apply{layers: []ispec.Descriptor{}, opts: opts}

	var source *umoci.Layout

	if opts.Layer.From.Type == DockerType || opts.Layer.From.Type == OCIType {
		var err error
		source, err = umoci.OpenLayout(path.Join(a.opts.Config.StackerDir, "layer-bases", "oci"))
		if err != nil {
			return nil, err
		}
		defer source.Close()
	} else if opts.Layer.From.Type == BuiltType {
		source = opts.OCI
	}

	if source != nil {
		tag, err := opts.Layer.From.ParseTag()
		if err != nil {
			return nil, err
		}

		manifest, err := source.LookupManifest(tag)
		if err != nil {
			return nil, err
		}

		for _, l := range manifest.Layers {
			a.layers = append(a.layers, l)
		}
	}

	return a, nil
}

func (a *Apply) ApplyLayer(layer string) error {
	err := runSkopeo(layer, a.opts, false)
	if err != nil {
		return err
	}

	oci, err := umoci.OpenLayout(path.Join(a.opts.Config.StackerDir, "layer-bases", "oci"))
	if err != nil {
		return err
	}
	defer oci.Close()

	tag, err := tagFromSkopeoUrl(layer)
	if err != nil {
		return err
	}

	manifest, err := oci.LookupManifest(tag)
	if err != nil {
		return err
	}

	for _, l := range manifest.Layers {
		// did we already extract this layer in this image?
		found := false
		for _, l2 := range a.layers {
			if l2.Digest == l.Digest {
				found = true
				break
			}
		}

		if found {
			continue
		}

		fmt.Println("applying layer", l.Digest)

		// apply the layer. TODO: we could be smart about this if the
		// layer is strictly additive or doesn't otherwise require
		// merging, we could realize that and add it directly to the
		// OCI output, so that it is kept as its own layer.
		err := applyLayer(oci, l, path.Join(a.opts.Config.RootFSDir, a.opts.Target, "rootfs"))
		if err != nil {
			return err
		}

		a.layers = append(a.layers, l)
	}

	return nil
}

func applyLayer(oci *umoci.Layout, desc ispec.Descriptor, target string) error {
	blob, err := oci.LookupBlob(desc)
	if err != nil {
		return err
	}
	defer blob.Close()

	var reader io.ReadCloser
	switch blob.MediaType {
	case ispec.MediaTypeImageLayer:
		reader = blob.Data.(io.ReadCloser)
		// closed by blob.Close()
	case ispec.MediaTypeImageLayerGzip:
		reader, err = pgzip.NewReader(blob.Data.(io.ReadCloser))
		if err != nil {
			return err
		}
		defer reader.Close()
	default:
		return fmt.Errorf("unknown layer type %s", blob.MediaType)
	}

	didMerge := false
	tr := tar.NewReader(reader)
	te := layer.NewTarExtractor(layer.MapOptions{})
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return errors.Wrapf(err, "apply layer")
		}

		// A slightly special case: we skip ., since the root directory
		// of the filesystem can be mounted with any number of
		// permissions which may not match.
		if hdr.Name == "." {
			continue
		}

		merged, err := insertOneFile(hdr, target, te, tr)
		if err != nil {
			return err
		}

		didMerge = didMerge || merged
	}

	if !didMerge {
		// TODO: here we can be very smart: we didn't need to merge
		// anything, so all the files were either new, or their
		// contents exactly matched the layers below. So we can just
		// inject the source layer above for this into the image,
		// instead of piling it on and generating a giant layer with
		// all of these merged files combined.
		//
		// Indeed, even if we did a merge, we could still inject this
		// old layer, since our rootfs reflects the merge when it is
		// repacked by umoci it will generate a new layer with the
		// right file content. Of course, this means the mostly same
		// file will occur multiple times in the archive, but since we
		// only do simple merging of text files, the files are
		// presumably small, and this is very cheap.
		//
		// But anyway, for now we just generate one giant blob because
		// it's less code.
	}

	return nil
}

func insertOneFile(hdr *tar.Header, target string, te *layer.TarExtractor, tr io.Reader) (bool, error) {
	fi, err := os.Lstat(path.Join(target, hdr.Name))
	if os.IsNotExist(err) {
		// if it didn't already exist, that's fine, just
		// process it normally
		return false, te.UnpackEntry(target, hdr, tr)
	} else if err != nil {
		return false, errors.Wrapf(err, "stat %s", path.Join(target, hdr.Name))
	}

	if fi.Mode() != hdr.FileInfo().Mode() {
		return false, fmt.Errorf("apply can't merge files of different types: %s", hdr.Name)
	}

	// zero is allowed, since umoci just picks time.Now(), they
	// probably won't match.
	if fi.ModTime() != hdr.ModTime && !hdr.ModTime.IsZero() {

		// liblxc impolitely binds its own init into /tmp/.lxc-init,
		// which changes the mtime on /tmp
		if hdr.Name == "tmp/" {
			return false, nil
		}

		// we bind the host's /etc/resolv.conf to inside the container
		if hdr.Name == "etc/" || hdr.Name == "etc/resolv.conf" {
			return false, nil
		}

		return false, fmt.Errorf("two different mod times on %s %v %v", hdr.Name, fi.ModTime(), hdr.ModTime)
	}

	sysStat := fi.Sys().(*syscall.Stat_t)
	// explicitly don't consider access time
	cSec, cNsec := sysStat.Ctim.Unix()
	ctime := time.Unix(cSec, cNsec)
	if ctime != hdr.ChangeTime && !hdr.ChangeTime.IsZero() {
		return false, fmt.Errorf("changed times differ on %s", hdr.Name)
	}

	if sysStat.Uid != uint32(hdr.Uid) {
		return false, fmt.Errorf("two different uids on %s: %v %v", hdr.Name, sysStat.Uid, hdr.Uid)
	}

	if sysStat.Gid != uint32(hdr.Gid) {
		return false, fmt.Errorf("two different gids on %s: %v %v", hdr.Name, sysStat.Gid, hdr.Gid)
	}

	sz, err := syscall.Listxattr(path.Join(target, hdr.Name), nil)
	if err == nil {
		xattrBuf := make([]byte, sz)
		_, err = syscall.Listxattr(path.Join(target, hdr.Name), xattrBuf)
		if err != nil {
			return false, err
		}

		start := 0
		xattrs := []string{}
		for i, c := range xattrBuf {
			if c == 0 {
				xattrs = append(xattrs, string(xattrBuf[start:i]))
				start = i + 1
			}
		}

		if len(xattrs) != len(hdr.Xattrs) {
			return false, fmt.Errorf("different xattrs for %s: %v %v", hdr.Name, xattrs, hdr.Xattrs)
		}

		for k, v := range hdr.Xattrs {
			found := false
			for _, xattr := range xattrs {
				if fmt.Sprintf("%s=%s", k, v) == xattr {
					found = true
					break
				}
			}

			if !found {
				return false, fmt.Errorf("different xattrs for %s, missing %s=%s", hdr.Name, k, v)
			}
		}

	} else if err != syscall.ENODATA {
		return false, err
	}

	switch hdr.Typeflag {
	case tar.TypeDir, tar.TypeFifo:
		// no-op, already exists and matches
		return false, nil
	case tar.TypeChar, tar.TypeBlock:
		if (hdr.FileInfo().Mode()&os.ModeCharDevice != 0) != (hdr.Typeflag == tar.TypeChar) {
			if uint32(hdr.Devmajor) != unix.Major(sysStat.Dev) || uint32(hdr.Devminor) != unix.Minor(sysStat.Dev) {
				return false, fmt.Errorf("device number mismatches for %s", hdr.Name)
			}
			return false, nil
		}

		return false, fmt.Errorf("block/char mismatch: %s", hdr.Name)
	case tar.TypeLink:
		// make sure this new hard link points to the same
		// place as the existing one.
		targetFI, err := os.Lstat(path.Join(target, hdr.Linkname))
		if err != nil {
			return false, err
		}

		targetIno := targetFI.Sys().(*syscall.Stat_t).Ino
		curIno := fi.Sys().(*syscall.Stat_t).Ino
		if targetIno != curIno {
			return false, fmt.Errorf("hard link %s would change location", hdr.Name)
		}

		return false, nil
	case tar.TypeSymlink:
		// make sure this new symlink points to the same place
		// as the existing one.
		linkname, err := os.Readlink(path.Join(target, hdr.Name))
		if err != nil {
			return false, err
		}

		if linkname != hdr.Linkname {
			return false, fmt.Errorf("%s would change symlink from %s to %s", hdr.Name, linkname, hdr.Linkname)
		}

		return false, nil
	case tar.TypeReg, tar.TypeRegA:
		// Now the fun one. We want to do a diff of this file
		// with the existing file and try and merge them
		// somehow. If they're not mergable, then we bail. Note
		// that we don't have to check file mode, since we
		// ensured they were the same above.

		// First, write the file next to the new one. This way
		// it's on the same device, so if they're huge and
		// mergable, we don't do lots of extra IO on the final
		// rename.
		f, err := ioutil.TempFile(path.Dir(path.Join(target, hdr.Name)), "stacker-apply")
		if err != nil {
			return false, err
		}
		defer f.Close()
		defer os.Remove(f.Name())

		h := sha256.New()
		w := io.MultiWriter(f, h)

		n, err := io.Copy(w, tr)
		if err != nil {
			return false, err
		}
		if n != hdr.Size {
			return false, fmt.Errorf("%s was bad size in tar file", hdr.Name)
		}

		existing, err := os.Open(path.Join(target, hdr.Name))
		if err != nil {
			return false, err
		}
		defer existing.Close()

		if hdr.Size == fi.Size() {
			existingH := sha256.New()
			_, err = io.Copy(existingH, existing)
			if err != nil {
				return false, err
			}

			_, err = existing.Seek(0, os.SEEK_SET)
			if err != nil {
				return false, err
			}

			// The files are equal, we're ok.
			if bytes.Equal(existingH.Sum([]byte{}), h.Sum([]byte{})) {
				return false, nil
			}
		}

		// Now we know the files aren't equal. We don't want to
		// try that hard to diff things, so let's make sure we
		// only diff text files.
		buf := make([]byte, 512)
		_, err = existing.Read(buf)
		if err != nil {
			return false, err
		}

		_, err = existing.Seek(0, os.SEEK_SET)
		if err != nil {
			return false, err
		}

		contentType := http.DetectContentType(buf)
		if !strings.HasPrefix(contentType, "text") {
			return false, fmt.Errorf("existing file different, can't diff %s of type %s", hdr.Name, contentType)
		}

		return true, fmt.Errorf("merging not implemented right now: %s", hdr.Name)
	default:
		return false, fmt.Errorf("unknown tar typeflag for %s", hdr.Name)
	}
}

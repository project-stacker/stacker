package stacker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/mutate"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/pkg/fseval"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/vbatts/go-mtree"
	"golang.org/x/sys/unix"

	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/squashfs"
)

type BuildArgs struct {
	Config                  StackerConfig
	LeaveUnladen            bool
	NoCache                 bool
	Substitute              []string
	OnRunFailure            string
	ApplyConsiderTimestamps bool
	LayerType               string
	Debug                   bool
	OrderOnly               bool
	SetupOnly               bool
}

func updateBundleMtree(rootPath string, newPath ispec.Descriptor) error {
	newName := strings.Replace(newPath.Digest.String(), ":", "_", 1) + ".mtree"

	infos, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return err
	}

	for _, fi := range infos {
		if !strings.HasSuffix(fi.Name(), ".mtree") {
			continue
		}

		return os.Rename(path.Join(rootPath, fi.Name()), path.Join(rootPath, newName))
	}

	return nil
}

func mkSquashfs(bundlepath, ocidir string, eps *squashfs.ExcludePaths) (io.ReadCloser, error) {
	// generate the squashfs in OCIDir, and then open it, read it from
	// there, and delete it.
	if err := os.MkdirAll(ocidir, 0755); err != nil {
		return nil, err
	}

	rootfsPath := path.Join(bundlepath, "rootfs")
	return squashfs.MakeSquashfs(ocidir, rootfsPath, eps)
}

func GenerateSquashfsLayer(name, author, bundlepath, ocidir string, oci casext.Engine) error {
	meta, err := umoci.ReadBundleMeta(bundlepath)
	if err != nil {
		return err
	}

	mtreeName := strings.Replace(meta.From.Descriptor().Digest.String(), ":", "_", 1)
	mtreePath := path.Join(bundlepath, mtreeName+".mtree")

	mfh, err := os.Open(mtreePath)
	if err != nil {
		return err
	}

	spec, err := mtree.ParseSpec(mfh)
	if err != nil {
		return err
	}

	fsEval := fseval.DefaultFsEval
	rootfsPath := path.Join(bundlepath, "rootfs")
	newDH, err := mtree.Walk(rootfsPath, nil, umoci.MtreeKeywords, fsEval)
	if err != nil {
		return errors.Wrapf(err, "couldn't mtree walk %s", rootfsPath)
	}

	diffs, err := mtree.CompareSame(spec, newDH, umoci.MtreeKeywords)
	if err != nil {
		return err
	}

	// This is a pretty massive hack, because there's no library for
	// generating squashfs images. However, mksquashfs does take a list of
	// files to exclude from the image. So we go through and accumulate a
	// list of these files.
	//
	// For missing files, since we're going to use overlayfs with
	// squashfs, we use overlayfs' mechanism for whiteouts, which is a
	// character device with device numbers 0/0. But since there's no
	// library for generating squashfs images, we have to write these to
	// the actual filesystem, and then remember what they are so we can
	// delete them later.
	missing := []string{}
	defer func() {
		for _, f := range missing {
			os.Remove(f)
		}
	}()

	paths := squashfs.NewExcludePaths()
	for _, diff := range diffs {
		switch diff.Type() {
		case mtree.Modified, mtree.Extra:
			p := path.Join(rootfsPath, diff.Path())
			paths.AddInclude(p, diff.New().IsDir())
		case mtree.Missing:
			p := path.Join(rootfsPath, diff.Path())
			missing = append(missing, p)
			paths.AddInclude(p, diff.Old().IsDir())
			if err := unix.Mknod(p, unix.S_IFCHR, int(unix.Mkdev(0, 0))); err != nil {
				if !os.IsNotExist(err) && err != unix.ENOTDIR {
					// No privilege to create device nodes. Create a .wh.$filename instead.
					dirname := path.Dir(diff.Path())
					fname := fmt.Sprintf(".wh.%s", path.Base(diff.Path()))
					whPath := path.Join(rootfsPath, dirname, fname)
					fd, err := os.Create(whPath)
					if err != nil {
						return errors.Wrapf(err, "couldn't mknod whiteout for %s", diff.Path())
					}
					fd.Close()
				}
			}
		case mtree.Same:
			paths.AddExclude(path.Join(rootfsPath, diff.Path()))
		}
	}

	tmpSquashfs, err := mkSquashfs(bundlepath, ocidir, paths)
	if err != nil {
		return err
	}
	defer tmpSquashfs.Close()

	desc, err := stackeroci.AddBlobNoCompression(oci, name, tmpSquashfs)
	if err != nil {
		return err
	}

	newName := strings.Replace(desc.Digest.String(), ":", "_", 1) + ".mtree"
	err = umoci.GenerateBundleManifest(newName, bundlepath, fsEval)
	if err != nil {
		return err
	}

	os.Remove(mtreePath)
	meta.From = casext.DescriptorPath{
		Walk: []ispec.Descriptor{desc},
	}
	err = umoci.WriteBundleMeta(bundlepath, meta)
	if err != nil {
		return err
	}

	return nil
}

// Builder is responsible for building the layers based on stackerfiles
type Builder struct {
	builtStackerfiles StackerFiles // Keep track of all the Stackerfiles which were built
	opts              *BuildArgs   // Build options
}

// NewBuilder initializes a new Builder struct
func NewBuilder(opts *BuildArgs) *Builder {
	return &Builder{
		builtStackerfiles: make(map[string]*Stackerfile, 1),
		opts:              opts,
	}
}

// Build builds a single stackerfile
func (b *Builder) Build(file string) error {
	opts := b.opts

	if opts.NoCache {
		os.RemoveAll(opts.Config.StackerDir)
	}

	sf, err := NewStackerfile(file, append(opts.Substitute, b.opts.Config.Substitutions()...))
	if err != nil {
		return err
	}

	s, err := NewStorage(opts.Config)
	if err != nil {
		return err
	}
	if !opts.LeaveUnladen {
		defer s.Detach()
	}

	order, err := sf.DependencyOrder()
	if err != nil {
		return err
	}

	var oci casext.Engine
	if _, statErr := os.Stat(opts.Config.OCIDir); statErr != nil {
		oci, err = umoci.CreateLayout(opts.Config.OCIDir)
	} else {
		oci, err = umoci.OpenLayout(opts.Config.OCIDir)
	}
	if err != nil {
		return err
	}
	defer oci.Close()

	// Add this stackerfile to the list of stackerfiles which were built
	b.builtStackerfiles[file] = sf
	buildCache, err := OpenCache(opts.Config, oci, b.builtStackerfiles)
	if err != nil {
		return err
	}

	// compute the git version for the directory that the stacker file is
	// in. we don't care if it's not a git directory, because in that case
	// we'll fall back to putting the whole stacker file contents in the
	// metadata.
	gitVersion, _ := GitVersion(sf.referenceDirectory)

	username := os.Getenv("SUDO_USER")

	if username == "" {
		user, err := user.Current()
		if err != nil {
			return err
		}

		username = user.Username
	}

	host, err := os.Hostname()
	if err != nil {
		return err
	}

	author := fmt.Sprintf("%s@%s", username, host)

	s.Delete(WorkingContainerName)
	for _, name := range order {
		l, ok := sf.Get(name)
		if !ok {
			return fmt.Errorf("%s not present in stackerfile?", name)
		}

		// if a container builds on another container in a stacker
		// file, we can't correctly render the dependent container's
		// filesystem, since we don't know what the output of the
		// parent build will be. so let's refuse to run in setup-only
		// mode in this case.
		if opts.SetupOnly && l.From.Type == BuiltType {
			return errors.Errorf("no built type layers (%s) allowed in setup mode", name)
		}

		fmt.Printf("preparing image %s...\n", name)
		s.Delete(name)

		// We need to run the imports first since we now compare
		// against imports for caching layers. Since we don't do
		// network copies if the files are present and we use rsync to
		// copy things across, hopefully this isn't too expensive.
		fmt.Println("importing files...")
		imports, err := l.ParseImport()
		if err != nil {
			return err
		}

		err = CleanImportsDir(opts.Config, name, imports, buildCache)
		if err != nil {
			return err
		}

		if err := Import(opts.Config, name, imports); err != nil {
			return err
		}

		// Need to check if the image has bind mounts, if the image has bind mounts,
		// it needs to be rebuilt regardless of the build cache
		// The reason is that tracking build cache for bind mounted folders
		// is too expensive, so we don't do it
		binds, err := l.ParseBinds()
		if err != nil {
			return err
		}

		cacheEntry, ok := buildCache.Lookup(name)
		if ok && (len(binds) == 0) {
			if l.BuildOnly {
				if cacheEntry.Name != name {
					err = s.Snapshot(cacheEntry.Name, name)
					if err != nil {
						return err
					}
				}
			} else {
				err = oci.UpdateReference(context.Background(), name, cacheEntry.Blob)
				if err != nil {
					return err
				}
			}
			fmt.Printf("found cached layer %s\n", name)
			continue
		}

		baseOpts := BaseLayerOpts{
			Config:    opts.Config,
			Name:      name,
			Target:    WorkingContainerName,
			Layer:     l,
			Cache:     buildCache,
			OCI:       oci,
			LayerType: opts.LayerType,
			Debug:     opts.Debug,
		}

		s.Delete(WorkingContainerName)
		if l.From.Type == BuiltType {
			if err := s.Restore(l.From.Tag, WorkingContainerName); err != nil {
				return err
			}
		} else {
			if err := s.Create(WorkingContainerName); err != nil {
				return err
			}
		}

		err = GetBaseLayer(baseOpts, b.builtStackerfiles)
		if err != nil {
			return err
		}

		apply, err := NewApply(b.builtStackerfiles, baseOpts, s, opts.ApplyConsiderTimestamps)
		if err != nil {
			return err
		}

		err = apply.DoApply()
		if err != nil {
			return err
		}

		c, err := NewContainer(opts.Config, WorkingContainerName)
		if err != nil {
			return err
		}
		defer c.Close()

		err = c.SetupLayerConfig(name, l)
		if err != nil {
			return err
		}

		if opts.SetupOnly {
			err = c.c.SaveConfigFile(path.Join(opts.Config.RootFSDir, WorkingContainerName, "lxc.conf"))
			if err != nil {
				return errors.Wrapf(err, "error saving config file for %s", name)
			}

			if err := s.Snapshot(WorkingContainerName, name); err != nil {
				return err
			}
			fmt.Printf("setup for %s complete\n", name)
			continue
		}

		fmt.Println("running commands...")

		run, err := l.ParseRun()
		if err != nil {
			return err
		}

		if len(run) != 0 {

			importsDir := path.Join(opts.Config.StackerDir, "imports", name)

			shebangLine := "#!/bin/sh -xe\n"
			if strings.HasPrefix(run[0], "#!") {
				shebangLine = ""
			} else {
				_, err = os.Stat(path.Join(opts.Config.RootFSDir, WorkingContainerName, "rootfs/bin/sh"))
				if err != nil {
					return fmt.Errorf("rootfs for %s does not have a /bin/sh", name)
				}
			}
			if err := ioutil.WriteFile(
				path.Join(importsDir, ".stacker-run.sh"),
				[]byte(shebangLine+strings.Join(run, "\n")+"\n"),
				0755); err != nil {
				return err
			}

			fmt.Println("running commands for", name)

			// These should all be non-interactive; let's ensure that.
			err = c.Execute("/stacker/.stacker-run.sh", nil)
			if err != nil {
				if opts.OnRunFailure != "" {
					err2 := c.Execute(opts.OnRunFailure, os.Stdin)
					if err2 != nil {
						fmt.Printf("failed executing %s: %s\n", opts.OnRunFailure, err2)
					}
				}
				return fmt.Errorf("run commands failed: %s", err)
			}
		}

		// This is a build only layer, meaning we don't need to include
		// it in the final image, as outputs from it are going to be
		// imported into future images. Let's just snapshot it and add
		// a bogus entry to our cache.
		if l.BuildOnly {
			if err := s.Snapshot(WorkingContainerName, name); err != nil {
				return err
			}

			fmt.Println("build only layer, skipping OCI diff generation")

			// A small hack: for build only layers, we keep track
			// of the name, so we can make sure it exists when
			// there is a cache hit. We should probably make this
			// into some sort of proper Either type.
			if err := buildCache.Put(name, ispec.Descriptor{}); err != nil {
				return err
			}
			continue
		}

		fmt.Println("generating layer for", name)
		switch opts.LayerType {
		case "tar":
			err = RunUmociSubcommand(opts.Config, opts.Debug, []string{
				"--tag", name,
				"--bundle-path", path.Join(opts.Config.RootFSDir, WorkingContainerName),
				"repack",
			})
			if err != nil {
				return err
			}
		case "squashfs":
			err = RunSquashfsSubcommand(opts.Config, opts.Debug, []string{
				"--bundle-path", path.Join(opts.Config.RootFSDir, WorkingContainerName),
				"--tag", name, "--author", author, "repack",
			})
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown layer type: %s", opts.LayerType)
		}
		descPaths, err := oci.ResolveReference(context.Background(), name)
		if err != nil {
			return err
		}

		mutator, err := mutate.New(oci, descPaths[0])
		if err != nil {
			return errors.Wrapf(err, "mutator failed")
		}

		imageConfig, err := mutator.Config(context.Background())
		if err != nil {
			return err
		}

		pathSet := false
		for k, v := range l.Environment {
			if k == "PATH" {
				pathSet = true
			}
			imageConfig.Env = append(imageConfig.Env, fmt.Sprintf("%s=%s", k, v))
		}

		if !pathSet {
			for _, s := range imageConfig.Env {
				if strings.HasPrefix(s, "PATH=") {
					pathSet = true
					break
				}
			}
		}

		// if the user didn't specify a path, let's set a sane one
		if !pathSet {
			imageConfig.Env = append(imageConfig.Env, fmt.Sprintf("PATH=%s", ReasonableDefaultPath))
		}

		if l.Cmd != nil {
			imageConfig.Cmd, err = l.ParseCmd()
			if err != nil {
				return err
			}
		}

		if l.Entrypoint != nil {
			imageConfig.Entrypoint, err = l.ParseEntrypoint()
			if err != nil {
				return err
			}
		}

		if l.FullCommand != nil {
			imageConfig.Cmd = nil
			imageConfig.Entrypoint, err = l.ParseFullCommand()
			if err != nil {
				return err
			}
		}

		if imageConfig.Volumes == nil {
			imageConfig.Volumes = map[string]struct{}{}
		}

		for _, v := range l.Volumes {
			imageConfig.Volumes[v] = struct{}{}
		}

		if imageConfig.Labels == nil {
			imageConfig.Labels = map[string]string{}
		}

		for k, v := range l.Labels {
			imageConfig.Labels[k] = v
		}

		if l.WorkingDir != "" {
			imageConfig.WorkingDir = l.WorkingDir
		}

		if l.RuntimeUser != "" {
			imageConfig.User = l.RuntimeUser
		}

		meta, err := mutator.Meta(context.Background())
		if err != nil {
			return err
		}

		meta.Created = time.Now()
		meta.Architecture = runtime.GOARCH
		meta.OS = runtime.GOOS
		meta.Author = author

		annotations, err := mutator.Annotations(context.Background())
		if err != nil {
			return err
		}

		if gitVersion != "" {
			fmt.Println("setting git version annotation to", gitVersion)
			annotations[GitVersionAnnotation] = gitVersion
		} else {
			annotations[StackerContentsAnnotation] = sf.AfterSubstitutions
		}

		history := ispec.History{
			EmptyLayer: true, // this is only the history for imageConfig edit
			Created:    &meta.Created,
			CreatedBy:  "stacker build",
			Author:     author,
		}

		err = mutator.Set(context.Background(), imageConfig, meta, annotations, &history)
		if err != nil {
			return err
		}

		newPath, err := mutator.Commit(context.Background())
		if err != nil {
			return err
		}

		err = oci.UpdateReference(context.Background(), name, newPath.Root())
		if err != nil {
			return err
		}

		// Now, we need to set the umoci data on the fs to tell it that
		// it has a layer that corresponds to this fs.
		bundlePath := path.Join(opts.Config.RootFSDir, WorkingContainerName)
		err = updateBundleMtree(bundlePath, newPath.Descriptor())
		if err != nil {
			return err
		}

		umociMeta := umoci.Meta{Version: umoci.MetaVersion, From: newPath}
		err = umoci.WriteBundleMeta(bundlePath, umociMeta)
		if err != nil {
			return err
		}

		if err := s.Snapshot(WorkingContainerName, name); err != nil {
			return err
		}

		fmt.Printf("filesystem %s built successfully\n", name)

		descPaths, err = oci.ResolveReference(context.Background(), name)
		if err != nil {
			return err
		}

		if err := buildCache.Put(name, descPaths[0].Descriptor()); err != nil {
			return err
		}
	}

	return oci.GC(context.Background())
}

// BuildMultiple builds a list of stackerfiles
func (b *Builder) BuildMultiple(paths []string) error {
	opts := b.opts

	// Read all the stacker recipes
	stackerFiles, err := NewStackerFiles(paths, append(opts.Substitute, b.opts.Config.Substitutions()...))
	if err != nil {
		return err
	}

	// Initialize the DAG
	dag, err := NewStackerFilesDAG(stackerFiles)
	if err != nil {
		return err
	}

	sortedPaths := dag.Sort()

	// Show the serial build order
	fmt.Printf("stacker build order:\n")
	for i, p := range sortedPaths {
		prerequisites, err := dag.GetStackerFile(p).Prerequisites()
		if err != nil {
			return err
		}
		fmt.Printf("%d build %s: requires: %v\n", i, p, prerequisites)
	}

	if opts.OrderOnly {
		// User has requested only to see the build order, so skipping the actual build
		return nil
	}

	// Build all Stackerfiles
	for i, p := range sortedPaths {
		fmt.Printf("building: %d %s\n", i, p)

		err = b.Build(p)
		if err != nil {
			return err
		}
	}

	return nil
}

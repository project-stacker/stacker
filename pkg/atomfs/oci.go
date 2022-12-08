package atomfs

import (
	"path"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	stackeroci "stackerbuild.io/stacker/pkg/oci"
)

type MountOCIOpts struct {
	OCIDir                 string
	MetadataPath           string
	Tag                    string
	Target                 string
	AllowMissingVerityData bool
}

func (c MountOCIOpts) AtomsPath(parts ...string) string {
	atoms := path.Join(c.OCIDir, "blobs", "sha256")
	return path.Join(append([]string{atoms}, parts...)...)
}

func (c MountOCIOpts) MountedAtomsPath(parts ...string) string {
	mounts := path.Join(c.MetadataPath, "mounts")
	return path.Join(append([]string{mounts}, parts...)...)
}

func BuildMoleculeFromOCI(opts MountOCIOpts) (Molecule, error) {
	oci, err := umoci.OpenLayout(opts.OCIDir)
	if err != nil {
		return Molecule{}, err
	}
	defer oci.Close()

	man, err := stackeroci.LookupManifest(oci, opts.Tag)
	if err != nil {
		return Molecule{}, err
	}

	atoms := []ispec.Descriptor{}
	atoms = append(atoms, man.Layers...)

	// The OCI spec says that the first layer should be the bottom most
	// layer. In overlay it's the top most layer. Since the atomfs codebase
	// is mostly a wrapper around overlayfs, let's keep things in our db in
	// the same order that overlay expects them, i.e. the first layer is
	// the top most. That means we need to reverse the order in which the
	// atoms were inserted, because they were backwards.
	//
	// It's also terrible that golang doesn't have a reverse function, but
	// that's a discussion for a different block comment.
	for i := len(atoms)/2 - 1; i >= 0; i-- {
		opp := len(atoms) - 1 - i
		atoms[i], atoms[opp] = atoms[opp], atoms[i]
	}

	return Molecule{Atoms: atoms, config: opts}, nil
}

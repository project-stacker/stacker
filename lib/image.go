package lib

import (
	"context"
	"io"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/daemon"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/pkg/errors"
)

var urlSchemes map[string]func(string) (types.ImageReference, error)

func RegisterURLScheme(scheme string, f func(string) (types.ImageReference, error)) {
	urlSchemes[scheme] = f
}

func init() {
	// These should only be things which have pure go dependencies. Things
	// with additional C dependencies (e.g. containers/image/storage)
	// should live in their own package, so people can choose to add those
	// deps or not.
	urlSchemes = map[string]func(string) (types.ImageReference, error){}
	RegisterURLScheme("oci", layout.ParseReference)
	RegisterURLScheme("docker", docker.ParseReference)
	RegisterURLScheme("docker-daemon", daemon.ParseReference)
}

func localRefParser(ref string) (types.ImageReference, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return nil, errors.Errorf("bad image ref: %s", ref)
	}

	f, ok := urlSchemes[parts[0]]
	if !ok {
		return nil, errors.Errorf("unknown url scheme %s for %s", parts[0], ref)
	}

	return f(parts[1])
}

type ImageCopyOpts struct {
	Src               string
	SrcUsername       string
	SrcPassword       string
	Dest              string
	DestUsername      string
	DestPassword      string
	ForceManifestType string
	SrcSkipTLS        bool
	DestSkipTLS       bool
	Progress          io.Writer
	Context           context.Context
}

func ImageCopy(opts ImageCopyOpts) error {
	if opts.Context == nil {
		opts.Context = context.Background()
	}

	srcRef, err := localRefParser(opts.Src)
	if err != nil {
		return err
	}

	destRef, err := localRefParser(opts.Dest)
	if err != nil {
		return err
	}

	policy, err := signature.NewPolicyContext(&signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	})
	if err != nil {
		return err
	}

	args := &copy.Options{
		ReportWriter:     opts.Progress,
		RemoveSignatures: true,
	}

	args.SourceCtx = &types.SystemContext{}

	if opts.SrcSkipTLS {
		args.SourceCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
		args.SourceCtx.DockerDaemonInsecureSkipTLSVerify = true
	}

	if opts.SrcUsername != "" {
		args.SourceCtx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: opts.SrcUsername,
			Password: opts.SrcPassword,
		}
	}

	args.DestinationCtx = &types.SystemContext{}

	if opts.DestSkipTLS {
		args.DestinationCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
		args.DestinationCtx.DockerDaemonInsecureSkipTLSVerify = true
	}

	if opts.DestUsername != "" {
		args.DestinationCtx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: opts.DestUsername,
			Password: opts.DestPassword,
		}
	}

	args.SourceCtx.OCIAcceptUncompressedLayers = true
	args.DestinationCtx.OCIAcceptUncompressedLayers = true

	// Set ForceManifestMIMEType
	// Supported manifest type :- https://github.com/containers/image/blob/master/manifest/manifest.go#L49
	// ImageCopy caller should set correct manifest type at its end.
	if opts.ForceManifestType != "" {
		args.ForceManifestMIMEType = opts.ForceManifestType
	}

	_, err = copy.Image(opts.Context, policy, destRef, srcRef, args)
	if err != nil {
		return err
	}

	// containers/image OCI as of
	// https://github.com/containers/image/commit/ca5fe04cb38a1f0e0b960e9388a3c6372efd215a
	// no longer deletes the old manifest from the index when it is
	// re-tagged, it just deletes the tag from the manifest and leaves the
	// manifest in the index untagged.
	//
	// umoci as of
	// https://github.com/opencontainers/umoci/commit/f5eda69b4f5a2e59773fd34ac0866a107a1dbb67
	// no longer ignores manifests in the index without tags when figuring
	// out what to GC.
	//
	// This means that when we do a copy and a subsequent GC with both deps
	// newer than the above hashes, the subsequent GC wouldn't do anything.
	//
	// Let's fix this by just deleting anything from the OCI repo that
	// doesn't have a valid tag after a copy.
	if destRef.Transport().Name() == "oci" {
		// oci:$path:$tag
		parts := strings.SplitN(opts.Dest, ":", 3)
		if len(parts) != 3 {
			return errors.Errorf("un-parsable oci dest %s", opts.Dest)
		}

		oci, err := umoci.OpenLayout(parts[1])
		if err != nil {
			return err
		}
		defer oci.Close()

		index, err := oci.GetIndex(opts.Context)
		if err != nil {
			return err
		}

		newIndex := []ispec.Descriptor{}
		for _, desc := range index.Manifests {
			name, ok := desc.Annotations[ispec.AnnotationRefName]
			if !ok || name == "" {
				continue
			}

			newIndex = append(newIndex, desc)
		}

		index.Manifests = newIndex
		err = oci.PutIndex(opts.Context, index)
		if err != nil {
			return err
		}
	}

	return nil
}

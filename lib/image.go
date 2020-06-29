package lib

import (
	"context"
	"io"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
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
	Src          string
	Dest         string
	DestUsername string
	DestPassword string
	SkipTLS      bool
	Progress     io.Writer
}

func ImageCopy(opts ImageCopyOpts) error {
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
		ReportWriter: opts.Progress,
	}

	if opts.SkipTLS {
		args.SourceCtx = &types.SystemContext{
			DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
		}
	}

	args.DestinationCtx = &types.SystemContext{}

	if opts.DestUsername != "" {
		args.DestinationCtx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: opts.DestUsername,
			Password: opts.DestPassword,
		}
	}

	_, err = copy.Image(context.Background(), policy, destRef, srcRef, args)
	return err
}

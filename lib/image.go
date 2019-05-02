package lib

import (
	"context"
	"io"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker"
	"github.com/containers/image/oci/layout"
	"github.com/containers/image/signature"
	"github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

func localRefParser(ref string) (types.ImageReference, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return nil, errors.Errorf("bad image ref: %s", ref)
	}

	switch parts[0] {
	case "oci":
		return layout.ParseReference(parts[1])
	case "docker":
		return docker.ParseReference(parts[1])
	case "containers-storage":
		return storage.Transport.ParseReference(parts[1])
	default:
		return nil, errors.Errorf("unknown image ref type: %s", ref)
	}
}

type ImageCopyOpts struct {
	Src      string
	Dest     string
	SkipTLS  bool
	Progress io.Writer
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

	// lol. and all this crap is the reason we make everyone install
	// libgpgme-dev, and we don't even want to use it :(
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

	_, err = copy.Image(context.Background(), policy, destRef, srcRef, args)
	return err
}

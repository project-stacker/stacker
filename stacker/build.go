package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/pkg/fseval"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

var buildCmd = cli.Command{
	Name:   "build",
	Usage:  "builds a new OCI image from a stacker yaml file",
	Action: usernsWrapper(doBuild),
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "leave-unladen",
			Usage: "leave the built rootfs mount after image building",
		},
		cli.StringFlag{
			Name:  "stacker-file, f",
			Usage: "the input stackerfile",
			Value: "stacker.yaml",
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "don't use the previous build cache",
		},
		cli.StringSliceFlag{
			Name:  "substitute",
			Usage: "variable substitution in stackerfiles, FOO=bar format",
		},
	},
}

func doBuild(ctx *cli.Context) error {
	if ctx.Bool("no-cache") {
		os.Remove(config.StackerDir)
	}

	file := ctx.String("f")
	sf, err := stacker.NewStackerfile(file)
	if err != nil {
		return err
	}

	for _, subst := range ctx.StringSlice("substitute") {
		membs := strings.SplitN(subst, "=", 2)
		if len(membs) != 2 {
			return fmt.Errorf("invalid substition %s", subst)
		}

		sf.VariableSub(membs[0], membs[1])
	}

	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	if !ctx.Bool("leave-unladen") {
		defer s.Detach()
	}

	order, err := sf.DependencyOrder()
	if err != nil {
		return err
	}

	var oci *umoci.Layout
	if _, err := os.Stat(config.OCIDir); err != nil {
		oci, err = umoci.CreateLayout(config.OCIDir)
	} else {
		oci, err = umoci.OpenLayout(config.OCIDir)
	}
	if err != nil {
		return err
	}
	defer oci.Close()

	buildCache, err := stacker.OpenCache(config.StackerDir)
	if err != nil {
		return err
	}

	defer s.Delete(".working")
	for _, name := range order {
		l := sf[name]

		_, ok := buildCache.Lookup(l)
		if ok {
			// TODO: for full correctness here we really need to
			// add a new tag with the current name for this layout,
			// in case someone changed the name instead.
			fmt.Printf("found cached layer %s\n", name)
			continue
		}

		s.Delete(".working")
		fmt.Printf("building image %s...\n", name)
		if l.From.Type == stacker.BuiltType {
			if err := s.Restore(l.From.Tag, ".working"); err != nil {
				return err
			}
		} else {
			if err := s.Create(".working"); err != nil {
				return err
			}

			os := stacker.BaseLayerOpts{
				Config: config,
				Name:   name,
				Target: ".working",
				Layer:  l,
				Cache:  buildCache,
				OCI:    oci,
			}

			err := stacker.GetBaseLayer(os)
			if err != nil {
				return err
			}
		}

		fmt.Println("importing files...")
		if err := stacker.Import(config, name, l.Import); err != nil {
			return err
		}

		fmt.Println("running commands...")
		if err := stacker.Run(config, name, l); err != nil {
			return err
		}

		// This is a build only layer, meaning we don't need to include
		// it in the final image, as outputs from it are going to be
		// imported into future images. Let's just snapshot it and add
		// a bogus entry to our cache.
		if l.BuildOnly {
			s.Delete(name)
			if err := s.Snapshot(".working", name); err != nil {
				return err
			}

			fmt.Println("build only layer, skipping OCI diff generation")
			if err := buildCache.Put(l, ispec.Descriptor{}); err != nil {
				return err
			}
			continue
		}

		fmt.Println("generating layer...")
		cmd := exec.Command(
			"umoci",
			"repack",
			"--image",
			fmt.Sprintf("%s:%s", config.OCIDir, name),
			path.Join(config.RootFSDir, ".working"))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error during repack: %s: %s", err, string(output))
		}

		mutator, err := oci.Mutator(name)
		if err != nil {
			return err
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
			imageConfig.Env = append(imageConfig.Env, fmt.Sprintf("PATH=%s", stacker.ReasonableDefaultPath))
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

		meta, err := mutator.Meta(context.Background())
		if err != nil {
			return err
		}

		meta.Created = time.Now()
		meta.Architecture = runtime.GOARCH
		meta.OS = runtime.GOOS

		annotations, err := mutator.Annotations(context.Background())
		if err != nil {
			return err
		}

		history := ispec.History{
			EmptyLayer: true, // this is only the history for imageConfig edit
			Created:    &meta.Created,
			CreatedBy:  "stacker build",
		}

		err = mutator.Set(context.Background(), imageConfig, meta, annotations, history)
		if err != nil {
			return err
		}

		newPath, err := mutator.Commit(context.Background())
		if err != nil {
			return err
		}

		err = oci.UpdateReference(name, newPath.Root())
		if err != nil {
			return err
		}

		// Now, we need to set the umoci data on the fs to tell it that
		// it has a layer that corresponds to this fs.
		mtreeName := strings.Replace(newPath.Descriptor().Digest.String(), ":", "_", 1)
		bundlePath := path.Join(config.RootFSDir, ".working")
		err = umoci.GenerateBundleManifest(mtreeName, bundlePath, fseval.DefaultFsEval)
		if err != nil {
			return err
		}

		// TODO: delete old mtree file

		umociMeta := umoci.UmociMeta{Version: umoci.UmociMetaVersion, From: newPath}
		err = umoci.WriteBundleMeta(bundlePath, umociMeta)
		if err != nil {
			return err
		}

		// Delete the old snapshot if it existed; we just did a new build.
		s.Delete(name)
		if err := s.Snapshot(".working", name); err != nil {
			return err
		}

		fmt.Printf("filesystem %s built successfully\n", name)

		manifest, err := oci.LookupManifest(name)
		if err != nil {
			return err
		}

		if err := buildCache.Put(l, manifest.Config); err != nil {
			return err
		}
	}

	return nil
}

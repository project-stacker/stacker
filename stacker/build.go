package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	igen "github.com/openSUSE/umoci/oci/config/generate"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

var buildCmd = cli.Command{
	Name:   "build",
	Usage:  "builds a new OCI image from a stacker yaml file",
	Action: doBuild,
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
		cli.BoolFlag{
			Name:  "btrfs-diff",
			Usage: "enable btrfs native layer diffing",
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

	buildCache, err := stacker.OpenCache(config.StackerDir, oci)
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
		if err := stacker.Run(config, name, l.Run); err != nil {
			return err
		}

		// Delete the old snapshot if it existed; we just did a new build.
		s.Delete(name)
		if err := s.Snapshot(".working", name); err != nil {
			return err
		}
		fmt.Printf("filesystem %s built successfully\n", name)

		diffType := stacker.TarDiff
		if ctx.Bool("btrfs-diff") {
			diffType = stacker.NativeDiff
		}

		diffSource := ""
		if l.From.Type == stacker.BuiltType {
			diffSource = l.From.Tag
		}

		diff, hash, err := s.Diff(diffType, diffSource, name)
		if err != nil {
			return err
		}
		defer diff.Close()

		fmt.Println("starting diff...")

		blob, err := oci.PutBlob(diff)
		if err != nil {
			return err
		}

		fmt.Printf("added blob %v\n", blob.Hash)
		diffID := blob.Hash
		if hash != nil {
			diffID = fmt.Sprintf("sha256:%x", hash.Sum(nil))
		}

		g := igen.New()
		g.SetCreated(time.Now())
		g.SetOS(runtime.GOOS)
		g.SetArchitecture(runtime.GOARCH)
		g.ClearHistory()

		g.SetRootfsType("layers")
		g.ClearRootfsDiffIDs()

		if l.Entrypoint != "" {
			cmd, err := l.ParseEntrypoint()
			if err != nil {
				return err
			}

			g.SetConfigEntrypoint(cmd)
		}

		// TODO: we should probably support setting environment
		// variables somehow, but for now let's set a sane PATH
		g.ClearConfigEnv()
		g.AddConfigEnv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin/bin")

		deps := []umoci.Blob{}

		mediaType := ispec.MediaTypeImageLayerGzip
		if ctx.Bool("btrfs-diff") {
			mediaType = stacker.MediaTypeImageBtrfsLayer
		}

		if l.From.Type == stacker.BuiltType {
			from, ok := sf[l.From.Tag]
			if !ok {
				return fmt.Errorf("didn't find parent %s in stackerfile", l.From.Tag)
			}

			fromBlob, ok := buildCache.Lookup(from)
			if !ok {
				return fmt.Errorf("didn't find parent %s in cache", l.From.Tag)
			}

			img, err := oci.LookupConfig(fromBlob)
			if err != nil {
				return err
			}

			for _, did := range img.RootFS.DiffIDs {
				g.AddRootfsDiffID(did)
			}

			manifest, err := oci.LookupManifest(l.From.Tag)
			if err != nil {
				return err
			}

			for _, l := range manifest.Layers {
				if mediaType != l.MediaType {
					return fmt.Errorf("media type mismatch: %s %s", mediaType, l.MediaType)
				}

				deps = append(deps, umoci.Blob{
					Hash: string(l.Digest),
					Size: l.Size,
				})
			}
		} else if l.From.Type == stacker.DockerType || l.From.Type == stacker.OCIType {
			tag, err := l.From.ParseTag()
			if err != nil {
				return err
			}

			// TODO: this is essentially the same as the above code
			manifest, err := oci.LookupManifest(tag)
			if err != nil {
				return err
			}

			configBlob := umoci.Blob{
				Hash: string(manifest.Config.Digest),
				Size: manifest.Config.Size,
			}

			img, err := oci.LookupConfig(configBlob)
			if err != nil {
				return err
			}

			for _, did := range img.RootFS.DiffIDs {
				g.AddRootfsDiffID(did)
			}

			for _, l := range manifest.Layers {
				if mediaType != l.MediaType {
					return fmt.Errorf("media type mismatch: %s %s", mediaType, l.MediaType)
				}

				deps = append(deps, umoci.Blob{
					Hash: string(l.Digest),
					Size: l.Size,
				})
			}
		}

		err = g.AddRootfsDiffIDStr(diffID)
		if err != nil {
			return err
		}

		deps = append(deps, blob)

		err = oci.NewImage(name, g, deps, mediaType)
		if err != nil {
			return err
		}

		manifest, err := oci.LookupManifest(name)
		if err != nil {
			return err
		}

		configBlob := umoci.Blob{
			Hash: string(manifest.Config.Digest),
			Size: manifest.Config.Size,
		}

		if err := buildCache.Put(l, configBlob); err != nil {
			return err
		}

	}

	return nil
}

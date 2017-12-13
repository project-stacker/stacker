package main

import (
	"fmt"
	"os"
	"runtime"
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

	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	if !ctx.Bool("leave-unladen") {
		defer s.Detach()
	}

	buildCache, err := openCache(config)
	if err != nil {
		return err
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

	defer s.Delete(".working")
	results := map[string]umoci.Layer{}

	for _, name := range order {
		l := sf[name]

		cached, ok := buildCache.Lookup(l)
		if ok {
			fmt.Printf("found cached layer %s\n", name)
			results[name] = cached
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

			err := stacker.GetBaseLayer(config, ".working", l)
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

		diff, err := s.Diff(diffType, diffSource, name)
		if err != nil {
			return err
		}
		defer diff.Close()

		fmt.Println("starting diff...")

		layer, err := oci.PutBlob(diff)
		if err != nil {
			return err
		}

		fmt.Printf("added blob %v\n", layer)
		results[name] = layer
		if err := buildCache.Put(l, layer); err != nil {
			return err
		}

		deps := []umoci.Layer{layer}
		for cur := l; cur.From.Type == stacker.BuiltType; cur = sf[cur.From.Tag] {
			deps = append([]umoci.Layer{results[cur.From.Tag]}, deps...)
		}

		g := igen.New()
		g.SetCreated(time.Now())
		g.SetOS(runtime.GOOS)
		g.SetArchitecture(runtime.GOARCH)
		g.ClearHistory()

		g.SetRootfsType("layers")
		g.ClearRootfsDiffIDs()

		for _, d := range deps {
			digest, err := d.ToDigest()
			if err != nil {
				return err
			}
			g.AddRootfsDiffID(digest)
		}

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

		mediaType := ispec.MediaTypeImageLayer
		if ctx.Bool("btrfs-diff") {
			mediaType = stacker.MediaTypeImageBtrfsLayer
		}

		if err := oci.NewImage(name, g, deps, mediaType); err != nil {
			return err
		}
	}

	return nil
}

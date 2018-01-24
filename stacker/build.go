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

		// Delete the old snapshot if it existed; we just did a new build.
		s.Delete(name)
		if err := s.Snapshot(".working", name); err != nil {
			return err
		}
		fmt.Printf("filesystem %s built successfully\n", name)

		mediaType := ispec.MediaTypeImageLayerGzip
		diffSource := ""
		if l.From.Type == stacker.BuiltType {
			diffSource = l.From.Tag
		}

		diff, hash, err := s.Diff(diffSource, name)
		if err != nil {
			return err
		}
		defer diff.Close()

		fmt.Println("starting diff...")

		digest, size, err := oci.PutBlob(diff)
		if err != nil {
			return err
		}

		blob := ispec.Descriptor{
			MediaType: mediaType,
			Digest:    digest,
			Size:      size,
		}

		fmt.Printf("added blob %v\n", string(digest))
		diffID := string(digest)
		if hash != nil {
			diffID = fmt.Sprintf("sha256:%x", hash.Sum(nil))
		}

		now := time.Now()

		g := igen.New()
		g.SetCreated(now)
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

		g.ClearConfigEnv()
		pathSet := false
		for k, v := range l.Environment {
			if k == "PATH" {
				pathSet = true
			}
			g.AddConfigEnv(k, v)
		}

		// if the user didn't specify a path, let's set a sane one
		if !pathSet {
			g.AddConfigEnv("PATH", stacker.ReasonableDefaultPath)
		}

		for _, v := range l.Volumes {
			g.AddConfigVolume(v)
		}

		for k, v := range l.Labels {
			g.AddConfigLabel(k, v)
		}

		if l.WorkingDir != "" {
			g.SetConfigWorkingDir(l.WorkingDir)
		}

		deps := []ispec.Descriptor{}

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

			for _, hist := range img.History {
				g.AddHistory(hist)
			}

			manifest, err := oci.LookupManifest(l.From.Tag)
			if err != nil {
				return err
			}

			for _, l := range manifest.Layers {
				if mediaType != l.MediaType {
					return fmt.Errorf("media type mismatch: %s %s", mediaType, l.MediaType)
				}

				deps = append(deps, l)
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

			img, err := oci.LookupConfig(manifest.Config)
			if err != nil {
				return err
			}

			for _, did := range img.RootFS.DiffIDs {
				g.AddRootfsDiffID(did)
			}

			for _, hist := range img.History {
				g.AddHistory(hist)
			}

			for _, l := range manifest.Layers {
				if mediaType != l.MediaType {
					return fmt.Errorf("media type mismatch: %s %s", mediaType, l.MediaType)
				}

				deps = append(deps, l)
			}
		}

		err = g.AddRootfsDiffIDStr(diffID)
		if err != nil {
			return err
		}

		g.AddHistory(ispec.History{
			Created:   &now,
			CreatedBy: "stacker",
		})

		deps = append(deps, blob)

		img := g.Image()
		platform := ispec.Platform{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
		}

		err = oci.NewImage(name, &img, deps, &platform)
		if err != nil {
			return err
		}

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

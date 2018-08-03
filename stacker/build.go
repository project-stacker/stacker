package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
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
		cli.StringFlag{
			Name:  "on-run-failure",
			Usage: "command to run inside container if run fails (useful for inspection)",
		},
		cli.BoolFlag{
			Name:  "shell-fail",
			Usage: "exec /bin/sh inside the container if run fails (alias for --on-run-failure=/bin/sh)",
		},
		cli.BoolFlag{
			Name:  "apply-consider-timestamps",
			Usage: "for apply layer merging, fail if timestamps on files don't match",
		},
	},
	Before: beforeBuild,
}

func beforeBuild(ctx *cli.Context) error {
	if ctx.Bool("shell-fail") {
		askedFor := ctx.String("on-run-failure")
		if askedFor != "" && askedFor != "/bin/sh" {
			return fmt.Errorf("--shell-fail is incompatible with --on-run-failure=%s", askedFor)
		}
		err := ctx.Set("on-run-failure", "/bin/sh")
		if err != nil {
			return err
		}
	}

	return nil
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

func doBuild(ctx *cli.Context) error {
	if ctx.Bool("no-cache") {
		os.RemoveAll(config.StackerDir)
	}

	file := ctx.String("f")
	sf, err := stacker.NewStackerfile(file, ctx.StringSlice("substitute"))
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

	order, err := sf.DependencyOrder()
	if err != nil {
		return err
	}

	var oci *umoci.Layout
	if _, statErr := os.Stat(config.OCIDir); statErr != nil {
		oci, err = umoci.CreateLayout(config.OCIDir)
	} else {
		oci, err = umoci.OpenLayout(config.OCIDir)
	}
	if err != nil {
		return err
	}
	defer oci.Close()

	buildCache, err := stacker.OpenCache(config, oci, sf)
	if err != nil {
		return err
	}

	dir, err := filepath.Abs(path.Dir(ctx.String("f")))
	if err != nil {
		return err
	}

	// compute the git version for the directory that the stacker file is
	// in. we don't care if it's not a git directory, because in that case
	// we'll fall back to putting the whole stacker file contents in the
	// metadata.
	gitVersion, _ := stacker.GitVersion(dir)

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

	s.Delete(".working")
	for _, name := range order {
		l, ok := sf.Get(name)
		if !ok {
			return fmt.Errorf("%s not present in stackerfile?", name)
		}

		fmt.Printf("building image %s...\n", name)

		// We need to run the imports first since we now compare
		// against imports for caching layers. Since we don't do
		// network copies if the files are present and we use rsync to
		// copy things across, hopefully this isn't too expensive.
		fmt.Println("importing files...")
		imports, err := l.ParseImport()
		if err != nil {
			return err
		}

		if err := stacker.Import(config, name, imports); err != nil {
			return err
		}

		cacheEntry, ok := buildCache.Lookup(name)
		if ok {
			if l.BuildOnly {
				if cacheEntry.Name != name {
					err = s.Snapshot(cacheEntry.Name, name)
					if err != nil {
						return err
					}
				}
			} else {
				err = oci.UpdateReference(name, cacheEntry.Blob)
				if err != nil {
					return err
				}
			}
			fmt.Printf("found cached layer %s\n", name)
			continue
		}

		os := stacker.BaseLayerOpts{
			Config: config,
			Name:   name,
			Target: ".working",
			Layer:  l,
			Cache:  buildCache,
			OCI:    oci,
		}

		s.Delete(".working")
		if l.From.Type == stacker.BuiltType {
			if err := s.Restore(l.From.Tag, ".working"); err != nil {
				return err
			}
		} else {
			if err := s.Create(".working"); err != nil {
				return err
			}
		}

		err = stacker.GetBaseLayer(os, sf)
		if err != nil {
			return err
		}

		apply, err := stacker.NewApply(sf, os, s, ctx.Bool("apply-consider-timestamps"))
		if err != nil {
			return err
		}

		err = apply.DoApply()
		if err != nil {
			return err
		}

		fmt.Println("running commands...")

		run, err := l.ParseRun()
		if err != nil {
			return err
		}

		if len(run) != 0 {
			importsDir := path.Join(config.StackerDir, "imports", name)

			script := fmt.Sprintf("#!/bin/bash -xe\n%s", strings.Join(run, "\n"))
			if err := ioutil.WriteFile(path.Join(importsDir, ".stacker-run.sh"), []byte(script), 0755); err != nil {
				return err
			}

			fmt.Println("running commands for", name)
			if err := stacker.Run(config, name, "/stacker/.stacker-run.sh", l, ctx.String("on-run-failure"), nil); err != nil {
				return err
			}
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

			// A small hack: for build only layers, we keep track
			// of the name, so we can make sure it exists when
			// there is a cache hit. We should probably make this
			// into some sort of proper Either type.
			if err := buildCache.Put(name, ispec.Descriptor{}); err != nil {
				return err
			}
			continue
		}

		fmt.Println("generating layer...")
		args := []string{
			"umoci",
			"repack",
			"--refresh-bundle",
			"--image",
			fmt.Sprintf("%s:%s", config.OCIDir, name),
			path.Join(config.RootFSDir, ".working")}
		err = stacker.MaybeRunInUserns(args, "layer generation failed")
		if err != nil {
			return err
		}

		mutator, err := oci.Mutator(name)
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
		meta.Author = author

		annotations, err := mutator.Annotations(context.Background())
		if err != nil {
			return err
		}

		if gitVersion != "" {
			fmt.Println("setting git version annotation to", gitVersion)
			annotations[stacker.GitVersionAnnotation] = gitVersion
		} else {
			annotations[stacker.StackerContentsAnnotation] = sf.AfterSubstitutions
		}

		history := ispec.History{
			EmptyLayer: true, // this is only the history for imageConfig edit
			Created:    &meta.Created,
			CreatedBy:  "stacker build",
			Author:     author,
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
		bundlePath := path.Join(config.RootFSDir, ".working")
		err = updateBundleMtree(bundlePath, newPath.Descriptor())
		if err != nil {
			return err
		}

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

		desc, err := oci.LookupManifestDescriptor(name)
		if err != nil {
			return err
		}

		if err := buildCache.Put(name, desc); err != nil {
			return err
		}
	}

	err = oci.GC()
	if err != nil {
		fmt.Printf("final OCI GC failed: %v", err)
	}

	return nil
}

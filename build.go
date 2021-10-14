package stacker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/types"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
)

type BuildArgs struct {
	Config       types.StackerConfig
	LeaveUnladen bool
	NoCache      bool
	Substitute   []string
	OnRunFailure string
	LayerTypes   []types.LayerType
	OrderOnly    bool
	SetupOnly    bool
	Progress     bool
}

// Builder is responsible for building the layers based on stackerfiles
type Builder struct {
	builtStackerfiles types.StackerFiles // Keep track of all the Stackerfiles which were built
	opts              *BuildArgs         // Build options
}

// NewBuilder initializes a new Builder struct
func NewBuilder(opts *BuildArgs) *Builder {
	return &Builder{
		builtStackerfiles: make(map[string]*types.Stackerfile, 1),
		opts:              opts,
	}
}

func (b *Builder) updateOCIConfigForOutput(sf *types.Stackerfile, s types.Storage, oci casext.Engine, layerType types.LayerType, l *types.Layer, name string) error {
	opts := b.opts

	layerName := layerType.LayerName(name)
	descPaths, err := oci.ResolveReference(context.Background(), layerName)
	if err != nil {
		return err
	}

	mutator, err := mutate.New(oci, descPaths[0])
	if err != nil {
		return errors.Wrapf(err, "mutator failed")
	}

	config, err := mutator.Config(context.Background())
	if err != nil {
		return err
	}

	imageConfig := config.Config

	if imageConfig.Labels == nil {
		imageConfig.Labels = map[string]string{}
	}

	generateLabels, err := l.ParseGenerateLabels()
	if err != nil {
		return err
	}

	if len(generateLabels) > 0 {
		writable, cleanup, err := s.TemporaryWritableSnapshot(name)
		if err != nil {
			return err
		}
		defer cleanup()

		dir, err := ioutil.TempDir(opts.Config.StackerDir, fmt.Sprintf("oci-labels-%s-", name))
		if err != nil {
			return errors.Wrapf(err, "failed to create oci-labels tempdir")
		}
		defer os.RemoveAll(dir)

		c, err := NewContainer(opts.Config, s, writable)
		if err != nil {
			return err
		}
		defer c.Close()

		err = c.bindMount(dir, "/oci-labels", "")
		if err != nil {
			return err
		}

		rootfs := path.Join(opts.Config.RootFSDir, writable, "rootfs")
		runPath := path.Join(dir, ".stacker-run.sh")
		err = GenerateShellForRunning(rootfs, generateLabels, runPath)
		if err != nil {
			return err
		}

		err = c.Execute("/oci-labels/.stacker-run.sh", nil)
		if err != nil {
			return err
		}

		ents, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s", dir)
		}

		for _, ent := range ents {
			if ent.Name() == ".stacker-run.sh" {
				continue
			}

			content, err := ioutil.ReadFile(path.Join(dir, ent.Name()))
			if err != nil {
				return errors.Wrapf(err, "couldn't read label %s", ent.Name())
			}

			imageConfig.Labels[ent.Name()] = string(content)
		}
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

	meta.Created = time.Now()
	meta.Architecture = runtime.GOARCH
	meta.OS = runtime.GOOS
	meta.Author = author

	annotations, err := mutator.Annotations(context.Background())
	if err != nil {
		return err
	}

	// compute the git version for the directory that the stacker file is
	// in. we don't care if it's not a git directory, because in that case
	// we'll fall back to putting the whole stacker file contents in the
	// metadata.
	gitVersion, _ := GitVersion(sf.ReferenceDirectory)

	if gitVersion != "" {
		log.Debugf("setting git version annotation to %s", gitVersion)
		annotations[GitVersionAnnotation] = gitVersion
	}

	annotations[StackerContentsAnnotation] = sf.AfterSubstitutions

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

	err = oci.UpdateReference(context.Background(), layerName, newPath.Root())
	if err != nil {
		return err
	}

	err = s.UpdateFSMetadata(name, newPath)
	if err != nil {
		return err
	}

	return nil
}

// Build builds a single stackerfile
func (b *Builder) Build(s types.Storage, file string) error {
	opts := b.opts

	if opts.NoCache {
		os.RemoveAll(opts.Config.StackerDir)
	}

	sf, err := types.NewStackerfile(file, append(opts.Substitute, b.opts.Config.Substitutions()...))
	if err != nil {
		return err
	}

	order, err := sf.DependencyOrder(b.builtStackerfiles)
	if err != nil {
		return err
	}

	log.Debugf("Dependency Order %v", order)

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

	for _, name := range order {
		l, ok := sf.Get(name)
		if !ok {
			return errors.Errorf("%s not present in stackerfile?", name)
		}

		// if a container builds on another container in a stacker
		// file, we can't correctly render the dependent container's
		// filesystem, since we don't know what the output of the
		// parent build will be. so let's refuse to run in setup-only
		// mode in this case.
		if opts.SetupOnly && l.From.Type == types.BuiltLayer {
			return errors.Errorf("no built type layers (%s) allowed in setup mode", name)
		}

		log.Infof("preparing image %s...", name)

		// We need to run the imports first since we now compare
		// against imports for caching layers. Since we don't do
		// network copies if the files are present and we use rsync to
		// copy things across, hopefully this isn't too expensive.
		imports, err := l.ParseImport()
		if err != nil {
			return err
		}

		err = CleanImportsDir(opts.Config, name, imports, buildCache)
		if err != nil {
			return err
		}

		if err := Import(opts.Config, s, name, imports, opts.Progress); err != nil {
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

		baseOpts := BaseLayerOpts{
			Config:     opts.Config,
			Name:       name,
			Layer:      l,
			Cache:      buildCache,
			OCI:        oci,
			LayerTypes: opts.LayerTypes,
			Storage:    s,
			Progress:   opts.Progress,
		}

		if err := GetBase(baseOpts); err != nil {
			return err
		}

		cacheEntry, cacheHit, err := buildCache.Lookup(name)
		if err != nil {
			return err
		}
		if cacheHit && (len(binds) == 0) {
			if l.BuildOnly {
				if cacheEntry.Name != name {
					err = s.Snapshot(cacheEntry.Name, name)
					if err != nil {
						return err
					}
				}
				continue
			} else {
				foundCount := 0
				for _, layerType := range opts.LayerTypes {
					blob, ok := cacheEntry.Manifests[layerType]
					if ok {
						foundCount += 1
						layerName := layerType.LayerName(name)
						err = oci.UpdateReference(context.Background(), layerName, blob)
						if err != nil {
							return err
						}
						log.Infof("found cached layer %s", layerName)
					}
				}

				if foundCount == len(opts.LayerTypes) {
					continue
				}

				log.Infof("missing some cached layer output types, building anyway")
			}
		} else if cacheHit && (len(binds) > 0) {
			log.Infof("rebuilding cached layer due to use of binds in stacker file")
		}

		err = SetupRootfs(baseOpts)
		if err != nil {
			return err
		}

		overlayDirs, err := l.ParseOverlayDirs()
		if err != nil {
			return err
		}

		err = s.SetOverlayDirs(name, overlayDirs, opts.LayerTypes)
		if err != nil {
			return err
		}

		c, err := NewContainer(opts.Config, s, name)
		if err != nil {
			return err
		}
		defer c.Close()

		err = c.SetupLayerConfig(l, name)
		if err != nil {
			return err
		}

		if opts.SetupOnly {
			err = c.c.SaveConfigFile(path.Join(opts.Config.RootFSDir, name, "lxc.conf"))
			if err != nil {
				return errors.Wrapf(err, "error saving config file for %s", name)
			}

			if err := s.Finalize(name); err != nil {
				return err
			}
			log.Infof("setup for %s complete", name)
			continue
		}

		run, err := l.ParseRun()
		if err != nil {
			return err
		}

		if len(run) != 0 {
			rootfs := path.Join(opts.Config.RootFSDir, name, "rootfs")
			shellScript := path.Join(opts.Config.StackerDir, "imports", name, ".stacker-run.sh")
			err = GenerateShellForRunning(rootfs, run, shellScript)
			if err != nil {
				return err
			}

			// These should all be non-interactive; let's ensure that.
			err = c.Execute("/stacker/.stacker-run.sh", nil)
			if err != nil {
				if opts.OnRunFailure != "" {
					err2 := c.Execute(opts.OnRunFailure, os.Stdin)
					if err2 != nil {
						log.Infof("failed executing %s: %s\n", opts.OnRunFailure, err2)
					}
				}
				return errors.Errorf("run commands failed: %s", err)
			}
		}

		// This is a build only layer, meaning we don't need to include
		// it in the final image, as outputs from it are going to be
		// imported into future images. Let's just snapshot it and add
		// a bogus entry to our cache.
		if l.BuildOnly {
			if err := s.Finalize(name); err != nil {
				return err
			}

			log.Debugf("build only layer, skipping OCI diff generation")

			// A small hack: for build only layers, we keep track
			// of the name, so we can make sure it exists when
			// there is a cache hit. We should probably make this
			// into some sort of proper Either type.
			manifests := map[types.LayerType]ispec.Descriptor{opts.LayerTypes[0]: ispec.Descriptor{}}
			if err := buildCache.Put(name, manifests); err != nil {
				return err
			}
			continue
		}

		err = s.Repack(name, opts.LayerTypes, b.builtStackerfiles)
		if err != nil {
			return err
		}

		manifests := map[types.LayerType]ispec.Descriptor{}
		for _, layerType := range opts.LayerTypes {
			err = b.updateOCIConfigForOutput(sf, s, oci, layerType, l, name)
			if err != nil {
				return err
			}

			descPaths, err := oci.ResolveReference(context.Background(), layerType.LayerName(name))
			if err != nil {
				return err
			}

			manifests[layerType] = descPaths[0].Descriptor()

		}

		if err := buildCache.Put(name, manifests); err != nil {
			return err
		}

		if err := s.Finalize(name); err != nil {
			return err
		}

		log.Infof("filesystem %s built successfully", name)

	}

	return oci.GC(context.Background())
}

// BuildMultiple builds a list of stackerfiles
func (b *Builder) BuildMultiple(paths []string) error {
	opts := b.opts

	s, err := NewStorage(opts.Config)
	if err != nil {
		return err
	}
	if !opts.LeaveUnladen {
		defer s.Detach()
	}

	// Read all the stacker recipes
	stackerFiles, err := types.NewStackerFiles(paths, append(opts.Substitute, b.opts.Config.Substitutions()...))
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
	log.Debugf("stacker build order:")
	for i, p := range sortedPaths {
		prerequisites, err := dag.GetStackerFile(p).Prerequisites()
		if err != nil {
			return err
		}
		log.Debugf("%d build %s: requires: %v", i, p, prerequisites)
	}

	if opts.OrderOnly {
		// User has requested only to see the build order, so skipping the actual build
		return nil
	}

	// Build all Stackerfiles
	for i, p := range sortedPaths {
		log.Debugf("building: %d %s", i, p)

		err = b.Build(s, p)
		if err != nil {
			return err
		}
	}

	return nil
}

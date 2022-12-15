package stacker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

type PublishArgs struct {
	Config     types.StackerConfig
	ShowOnly   bool
	Substitute []string
	Tags       []string
	Url        string
	Username   string
	Password   string
	Force      bool
	Progress   bool
	SkipTLS    bool
	LayerTypes []types.LayerType
}

// Publisher is responsible for publishing the layers based on stackerfiles
type Publisher struct {
	stackerfiles types.StackerFiles // Keep track of all the Stackerfiles to publish
	opts         *PublishArgs       // Publish options
}

// NewPublisher initializes a new Publisher struct
func NewPublisher(opts *PublishArgs) *Publisher {
	return &Publisher{
		stackerfiles: make(map[string]*types.Stackerfile, 1),
		opts:         opts,
	}
}

// Publish layers in a single stackerfile
func (p *Publisher) Publish(file string) error {
	opts := p.opts

	// Use absolute path to identify the file in stackerfile map
	absPath, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	sf, ok := p.stackerfiles[absPath]
	if !ok {
		return errors.Errorf("could not find entry for %s(%s) in stackerfiles", absPath, file)
	}

	var oci casext.Engine
	oci, err = umoci.OpenLayout(opts.Config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	buildCache, err := OpenCache(opts.Config, oci, p.stackerfiles)
	if err != nil {
		return err
	}

	// Determine list of tags to be used
	tags := make([]string, len(opts.Tags))
	copy(tags, opts.Tags)

	if len(tags) == 0 {
		return errors.Errorf("can't save OCI images in %s since list of tags is empty\n", file)
	}

	// Need to determine if URL is docker/oci or something else
	is, err := types.NewImageSource(opts.Url)
	if err != nil {
		return err
	}

	// Iterate through all layers defined in this stackerfile
	for _, name := range sf.FileOrder {

		// Verify layer is not build only
		l, ok := sf.Get(name)
		if !ok {
			return errors.Errorf("layer cannot be found in stackerfile: %s", name)
		}
		if l.BuildOnly {
			log.Infof("will not publish: %s build_only %s", file, name)
			continue
		}

		// Verify layer is in build cache
		_, ok, err = buildCache.Lookup(name)
		if err != nil {
			return err
		}
		if !ok && !opts.Force {
			return errors.Errorf("layer needs to be rebuilt before publishing: %s", name)
		}

		// Iterate through all tags
		for _, tag := range tags {
			for _, layerType := range opts.LayerTypes {
				layerTypeTag := layerType.LayerName(tag)
				layerName := layerType.LayerName(name)
				// Determine full destination URL
				var destUrl string
				switch is.Type {
				case types.DockerLayer:
					destUrl = fmt.Sprintf("%s/%s:%s", strings.TrimRight(opts.Url, "/"), name, layerTypeTag)
				case types.OCILayer:
					destUrl = fmt.Sprintf("%s:%s_%s", opts.Url, name, layerTypeTag)
				default:
					return errors.Errorf("can't save layers to destination type: %s", is.Type)
				}

				if opts.ShowOnly {
					// User has requested only to see what would be published
					log.Infof("would publish: %s %s to %s", file, name, destUrl)
					continue
				}

				var progressWriter io.Writer
				if p.opts.Progress {
					progressWriter = os.Stderr
				}

				// Store the layers to new destination
				log.Infof("publishing %s %s to %s\n", file, layerName, destUrl)
				err = lib.ImageCopy(lib.ImageCopyOpts{
					Src:          fmt.Sprintf("oci:%s:%s", opts.Config.OCIDir, layerName),
					Dest:         destUrl,
					DestUsername: opts.Username,
					DestPassword: opts.Password,
					Progress:     progressWriter,
					SrcSkipTLS:   true,
					DestSkipTLS:  opts.SkipTLS,
				})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// PublishMultiple published layers defined in a list of stackerfiles
func (p *Publisher) PublishMultiple(paths []string) error {

	// Verify the OCI layout exists
	if _, err := os.Stat(p.opts.Config.OCIDir); err != nil {
		return err
	}

	// Read stackerfiles and update substitutions
	sfm, err := p.readStackerFiles(paths)
	if err != nil {
		return err
	}
	p.stackerfiles = sfm

	// Publish all Stackerfiles
	for _, path := range paths {
		err := p.Publish(path)
		if err != nil {
			return err
		}
	}

	return nil
}

// readStackerFiles reads stacker recipes and applies substitutions
// it has a hack for determining if a value is not substituted
// if it should be substituted but is is not, substitute it with 'dummy'
func (p *Publisher) readStackerFiles(paths []string) (types.StackerFiles, error) {

	// Read all the stacker recipes
	sfm, err := types.NewStackerFiles(paths, false, append(p.opts.Substitute, p.opts.Config.Substitutions()...))
	if err != nil {

		// Verify if the error is related to an invalid substitution
		re := regexp.MustCompile(`no value for substitution (.*)`)
		matches := re.FindAllStringSubmatch(err.Error(), -1)

		// If the error is not related to an invalid substitution, report it
		if len(matches) == 0 {
			return nil, err
		}

		// If the error is related to an invalid substitution,
		// determine the missing variable and add it to the variable to substitute
		if len(matches[0]) < 2 {
			// For some strange reason the first capturing group has not caught anything
			return nil, err
		}

		// Add the value dummy to the missing substitute variables
		p.opts.Substitute = append(p.opts.Substitute, fmt.Sprintf("%s=dummy", matches[0][1]))

		// Try again, this time with the new substitute variables
		return p.readStackerFiles(paths)
	}
	return sfm, nil
}

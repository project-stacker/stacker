package stacker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/casext"

	"github.com/anuvu/stacker/lib"
)

type PublishArgs struct {
	Config     StackerConfig
	Debug      bool
	ShowOnly   bool
	Substitute []string
	Tags       []string
	Url        string
	Username   string
	Password   string
	Force      bool
}

// Publisher is responsible for publishing the layers based on stackerfiles
type Publisher struct {
	stackerfiles StackerFiles // Keep track of all the Stackerfiles to publish
	opts         *PublishArgs // Publish options
}

// NewPublisher initializes a new Publisher struct
func NewPublisher(opts *PublishArgs) *Publisher {
	return &Publisher{
		stackerfiles: make(map[string]*Stackerfile, 1),
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
		return fmt.Errorf("could not find entry for %s(%s) in stackerfiles", absPath, file)
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

	// Attempt to produce a git commit tag
	if ct, err := NewGitLayerTag(sf.referenceDirectory); err == nil {
		// Add git tag to the list of tags to be used
		tags = append(tags, ct)
	}

	if len(tags) == 0 {
		fmt.Printf("can't save OCI images in %s since list of tags is empty\n", file)
	}

	// Need to determine if URL is docker/oci or something else
	is, err := NewImageSource(opts.Url)
	if err != nil {
		return err
	}

	// Iterate through all layers defined in this stackerfile
	for _, name := range sf.fileOrder {

		// Verify layer is in build cache
		_, ok := buildCache.Lookup(name)
		if !ok && !opts.Force {
			return fmt.Errorf("layer needs to be rebuilt before publishing: %s", name)
		}

		// Iterate through all tags
		for _, tag := range tags {
			// Determine full destination URL
			var destUrl string
			switch is.Type {
			case DockerType:
				destUrl = fmt.Sprintf("%s/%s:%s", strings.TrimRight(opts.Url, "/"), name, tag)
			case OCIType:
				destUrl = fmt.Sprintf("%s:%s_%s", opts.Url, name, tag)
			case ZotType:
				destUrl = fmt.Sprintf("%s/%s:%s", strings.TrimRight(opts.Url, "/"), name, tag)
			default:
				return fmt.Errorf("can't save layers to destination type: %s", is.Type)
			}

			if opts.ShowOnly {
				// User has requested only to see what would be published
				fmt.Printf("would publish: %s %s to %s\n", file, name, destUrl)
				continue
			}

			// Store the layers to new destination
			fmt.Printf("publishing %s %s to %s\n", file, name, destUrl)
			err = lib.ImageCopy(lib.ImageCopyOpts{
				Src:          fmt.Sprintf("oci:%s:%s", opts.Config.OCIDir, name),
				Dest:         destUrl,
				DestUsername: opts.Username,
				DestPassword: opts.Password,
				Progress:     os.Stdout,
				SkipTLS:      true,
			})
			if err != nil {
				return err
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
func (p *Publisher) readStackerFiles(paths []string) (StackerFiles, error) {

	// Read all the stacker recipes
	sfm, err := NewStackerFiles(paths, p.opts.Substitute)
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

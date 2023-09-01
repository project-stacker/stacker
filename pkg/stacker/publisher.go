package stacker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	godigest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

type PublishArgs struct {
	Config         types.StackerConfig
	ShowOnly       bool
	SubstituteFile string
	Substitute     []string
	Tags           []string
	Url            string
	Username       string
	Password       string
	Force          bool
	Progress       bool
	SkipTLS        bool
	LayerTypes     []types.LayerType
	Layers         []string
}

// Publisher is responsible for publishing the layers based on stackerfiles
type Publisher struct {
	stackerfiles types.StackerFiles // Keep track of all the Stackerfiles to publish
	opts         *PublishArgs       // Publish options
}

// NewPublisher initializes a new Publisher struct
func NewPublisher(opts *PublishArgs) *Publisher {
	if opts.SubstituteFile != "" {
		bytes, err := os.ReadFile(opts.SubstituteFile)
		if err != nil {
			log.Fatalf("unable to read substitute-file:%s, err:%e", opts.SubstituteFile, err)
			return nil
		}

		var yamlMap map[string]string
		if err := yaml.Unmarshal(bytes, &yamlMap); err != nil {
			log.Fatalf("unable to unmarshal substitute-file:%s, err:%s", opts.SubstituteFile, err)
			return nil
		}

		for k, v := range yamlMap {
			opts.Substitute = append(opts.Substitute, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return &Publisher{
		stackerfiles: make(map[string]*types.Stackerfile, 1),
		opts:         opts,
	}
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func clientRequest(method, url, username, password string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.TODO(), method, url, body)
	if err != nil {
		log.Errorf("unable to create http request err:%s", err)
		return nil, err
	}

	// FIXME: handle bearer auth also
	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+basicAuth(username, password))
	}

	if len(headers) > 0 {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("http request failed url:%s", url)
		return nil, err
	}

	return res, nil
}

func fileDigest(path string) (*godigest.Digest, error) {
	fh, err := os.Open(path)
	if err != nil {
		log.Errorf("unable to open file:%s, err:%s", path, err)
		return nil, err
	}
	defer fh.Close()

	dgst, err := godigest.FromReader(fh)
	if err != nil {
		log.Errorf("unable get digest for file:%s, err:%s", path, err)
		return nil, err
	}

	return &dgst, nil
}

// publishArtifact to a registry/repo for this subject
func (p *Publisher) publishArtifact(path, mtype, registry, repo, subjectTag string, skipTLS bool) error {
	username := p.opts.Username
	password := p.opts.Password

	subject := distspecURL(registry, repo, subjectTag, skipTLS)

	// check subject exists
	res, err := clientRequest(http.MethodHead, subject, username, password, nil, nil)
	if err != nil {
		log.Errorf("unable to check subject:%s, err:%s", subject, err)
		return err
	}
	if res == nil || res.StatusCode != http.StatusOK {
		log.Errorf("subject:%s doesn't exist, ignoring and proceeding", subject)
	}

	slen := res.ContentLength
	smtype := res.Header.Get("Content-Type")
	sdgst, err := godigest.Parse(res.Header.Get("Docker-Content-Digest"))
	if slen < 0 || smtype == "" || sdgst == "" || err != nil {
		log.Errorf("unable to get descriptor details for subject:%s", subject)
		return errors.Errorf("unable to get descriptor details for subject:%s", subject)
	}

	// upload the artifact
	finfo, err := os.Lstat(path)
	if err != nil {
		log.Errorf("unable to stat file:%s, err:%s", path, err)
		return err
	}

	dgst, err := fileDigest(path)
	if err != nil {
		log.Errorf("unable get digest for file:%s, err:%s", path, err)
		return err
	}

	fh, err := os.Open(path)
	if err != nil {
		log.Errorf("unable to open file:%s, err:%s", path, err)
		return err
	}
	defer fh.Close()

	if err := uploadBlob(registry, repo, path, username, password, fh, finfo.Size(), dgst, skipTLS); err != nil {
		log.Errorf("unable to upload file:%s, err:%s", path, err)
		return err
	}

	// check and upload emptyJSON blob
	erdr := bytes.NewReader(ispec.DescriptorEmptyJSON.Data)
	edgst := ispec.DescriptorEmptyJSON.Digest

	if err := uploadBlob(registry, repo, path, username, password, erdr, ispec.DescriptorEmptyJSON.Size, &edgst, skipTLS); err != nil {
		log.Errorf("unable to upload file:%s, err:%s", path, err)
		return err
	}

	// upload the reference manifest
	manifest := ispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:    ispec.MediaTypeImageManifest,
		ArtifactType: mtype,
		Config:       ispec.DescriptorEmptyJSON,
		Subject: &ispec.Descriptor{
			MediaType: ispec.MediaTypeImageManifest,
			Size:      slen,
			Digest:    sdgst,
		},
		Layers: []ispec.Descriptor{
			ispec.Descriptor{
				MediaType: mtype,
				Size:      finfo.Size(),
				Digest:    *dgst,
			},
		},
	}

	//content, err := json.MarshalIndent(&manifest, "", "\t")
	content, err := json.Marshal(&manifest)
	if err != nil {
		log.Errorf("unable to marshal image manifest, err:%s", err)
		return err
	}

	// artifact manifest
	var regUrl string
	mdgst := godigest.FromBytes(content)
	if skipTLS {
		regUrl = fmt.Sprintf("http://%s/v2%s/manifests/%s", registry, strings.Split(repo, ":")[0], mdgst.String())
	} else {
		regUrl = fmt.Sprintf("https://%s/v2%s/manifests/%s", registry, strings.Split(repo, ":")[0], mdgst.String())
	}
	hdrs := map[string]string{
		"Content-Type":   ispec.MediaTypeImageManifest,
		"Content-Length": fmt.Sprintf("%d", len(content)),
	}
	res, err = clientRequest(http.MethodPut, regUrl, username, password, hdrs, bytes.NewBuffer(content))
	if err != nil {
		log.Errorf("unable to check subject:%s, err:%s", subject, err)
		return err
	}
	if res == nil || res.StatusCode != http.StatusCreated {
		log.Errorf("unable to upload manifest, url:%s", regUrl)
		return errors.Errorf("unable to upload manifest, url:%s", regUrl)
	}

	log.Infof("Copying artifact '%s' done", path)

	return nil
}

func distspecURL(registry, repo, tag string, skipTLS bool) string {
	var url string

	if skipTLS {
		url = fmt.Sprintf("http://%s/v2%s/manifests/%s", registry, strings.Split(repo, ":")[0], tag)
	} else {
		url = fmt.Sprintf("https://%s/v2%s/manifests/%s", registry, strings.Split(repo, ":")[0], tag)
	}

	return url
}

func uploadBlob(registry, repo, path, username, password string, reader io.Reader, size int64, dgst *godigest.Digest, skipTLS bool) error {
	// upload with POST, PUT sequence
	var regUrl string
	if skipTLS {
		regUrl = fmt.Sprintf("http://%s/v2%s/blobs/%s", registry, strings.Split(repo, ":")[0], dgst.String())
	} else {
		regUrl = fmt.Sprintf("https://%s/v2%s/blobs/%s", registry, strings.Split(repo, ":")[0], dgst.String())
	}

	subject := distspecURL(registry, repo, "", skipTLS)

	log.Debugf("check blob before upload (HEAD): %s", regUrl)
	res, err := clientRequest(http.MethodHead, regUrl, username, password, nil, nil)
	if err != nil {
		log.Errorf("unable to check blob:%s, err:%s", subject, err)
		return err
	}
	log.Debugf("http response headers: +%v status:%v", res.Header, res.Status)
	hdr := res.Header.Get("Docker-Content-Digest")
	if hdr != "" {
		log.Infof("Copying blob %s skipped: already exists", dgst.Hex()[:12])
		return nil
	}

	if skipTLS {
		regUrl = fmt.Sprintf("http://%s/v2%s/blobs/uploads/", registry, strings.Split(repo, ":")[0])
	} else {
		regUrl = fmt.Sprintf("https://%s/v2%s/blobs/uploads/", registry, strings.Split(repo, ":")[0])
	}

	log.Debugf("new blob upload (POST): %s", regUrl)
	res, err = clientRequest(http.MethodPost, regUrl, username, password, nil, nil)
	if err != nil {
		log.Errorf("post unable to check subject:%s, err:%s", subject, err)
		return err
	}
	log.Debugf("http response headers: +%v status:%v", res.Header, res.Status)
	loc, err := res.Location()
	if err != nil {
		log.Errorf("unable get upload location url:%s, err:%s", regUrl, err)
		return err
	}

	log.Debugf("finish blob upload (PUT): %s", regUrl)
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodPut, loc.String(), reader)
	if err != nil {
		log.Errorf("unable to create a http request url:%s", subject)
		return err
	}
	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+basicAuth(username, password))
	}
	req.URL.RawQuery = url.Values{
		"digest": {dgst.String()},
	}.Encode()

	req.ContentLength = size

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("http request failed url:%s", subject)
		return err
	}
	if res == nil || res.StatusCode != http.StatusCreated {
		log.Errorf("unable to upload artifact:%s to url:%s", path, regUrl)
		return errors.Errorf("unable to upload artifact:%s to url:%s", path, regUrl)
	}

	log.Infof("Copying blob %s done", dgst.Hex()[:12])

	return nil
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
		if len(p.opts.Layers) > 0 {
			found := false
			for _, lname := range p.opts.Layers {
				if lname == name {
					found = true
					break
				}
			}

			if !found {
				continue
			}
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

				if is.Type == types.DockerLayer && l.Bom != nil && l.Bom.Generate {
					url, err := types.NewDockerishUrl(destUrl)
					if err != nil {
						return err
					}

					registry := url.Host
					repo := url.Path

					// publish sbom
					if err := p.publishArtifact(path.Join(opts.Config.StackerDir, "artifacts", layerName, fmt.Sprintf("%s.json", layerName)),
						"application/spdx+json", registry, repo, layerTypeTag, opts.SkipTLS); err != nil {
						return err
					}

					// publish inventory
					if err := p.publishArtifact(path.Join(opts.Config.StackerDir, "artifacts", layerName, "inventory.json"),
						"application/vnd.stackerbuild.inventory+json", registry, repo, layerTypeTag, opts.SkipTLS); err != nil {
						return err
					}

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

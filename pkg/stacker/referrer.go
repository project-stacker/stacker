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
	"path/filepath"
	"strings"

	godigest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/log"
)

type distspecUrl struct {
	Scheme string
	Host   string
	Tag    string
	Path   string
}

func dumpHTTPHeaders(hdrs map[string][]string) string {
	ret := ""
	for k, v := range hdrs {
		ret += fmt.Sprintf("%s:%v,", k, v)
	}

	return ret
}

func parseDistSpecUrl(thing string) (distspecUrl, error) {
	parts := strings.SplitN(thing, "://", 2)

	if len(parts) == 1 {
		// oci: etc
		return distspecUrl{}, errors.Errorf("invalid url scheme: %s", parts[0])
	}

	prefix := "/"

	url := distspecUrl{Scheme: parts[0]}
	pathSplit := strings.SplitN(parts[1], "/", 2)
	var tagSplit []string
	if len(pathSplit) == 1 {
		url.Host = "docker.io"
		prefix = "/library"
		tagSplit = strings.SplitN(pathSplit[0], ":", 2)
	} else {
		url.Host = pathSplit[0]
		tagSplit = strings.SplitN(pathSplit[1], ":", 2)
	}

	if len(tagSplit) == 2 {
		url.Path = filepath.Join(prefix, tagSplit[0])
		url.Tag = tagSplit[1]
	} else {
		url.Path = filepath.Join("/", pathSplit[0])
		url.Tag = "latest"
	}

	return url, nil
}

const artifactTypeSPDX = "application/spdx+json"

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

	// NOTE: currently support only BASIC authN
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
func publishArtifact(path, mtype, registry, repo, subjectTag, username, password string, skipTLS bool) error {

	subject := distspecManifestURL(registry, strings.Split(repo, ":")[0], subjectTag, skipTLS)

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
	mdgst := godigest.FromBytes(content)
	regUrl := distspecManifestURL(registry, strings.Split(repo, ":")[0], mdgst.String(), skipTLS)
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

	log.Infof("Copying artifact %s done", path)

	return nil
}

func distspecScheme(skipTLS bool) string {
	if skipTLS {
		return "http"
	} else {
		return "https"
	}
}

func distspecManifestURL(registry, repo, tag string, skipTLS bool) string {
	return fmt.Sprintf("%s://%s/v2%s/manifests/%s", distspecScheme(skipTLS), registry, repo, tag)
}

func distspecBlobURL(registry, repo, tag string, skipTLS bool) string {
	return fmt.Sprintf("%s://%s/v2%s/blobs/%s", distspecScheme(skipTLS), registry, repo, tag)
}

func distspecBlobUploadURL(registry, repo string, skipTLS bool) string {
	return fmt.Sprintf("%s://%s/v2%s/blobs/uploads/", distspecScheme(skipTLS), registry, strings.Split(repo, ":")[0])
}

func distspecReferrerURL(registry, repo, tag, artifactType string, skipTLS bool) string {
	rurl := fmt.Sprintf("%s://%s/v2%s/referrers/%s", distspecScheme(skipTLS), registry, repo, tag)

	aurl, err := url.Parse(rurl)
	if err != nil {
		return ""
	}

	params := aurl.Query()
	params.Set("artifactType", artifactType)
	aurl.RawQuery = params.Encode()
	rurl = aurl.String()

	return rurl
}

func uploadBlob(registry, repo, path, username, password string, reader io.Reader, size int64, dgst *godigest.Digest, skipTLS bool) error {
	// upload with POST, PUT sequence
	regUrl := distspecManifestURL(registry, strings.Split(repo, ":")[0], dgst.String(), skipTLS)

	subject := distspecManifestURL(registry, repo, "", skipTLS)

	log.Debugf("Check blob before upload (HEAD): %s", regUrl)
	res, err := clientRequest(http.MethodHead, regUrl, username, password, nil, nil)
	if err != nil {
		log.Errorf("unable to check blob:%s, err:%s", subject, err)
		return err
	}
	log.Debugf("HTTP response status:%v headers:%v", res.Status, dumpHTTPHeaders(res.Header))

	hdr := res.Header.Get("Docker-Content-Digest")
	if hdr != "" {
		log.Infof("Copying blob %s skipped: already exists", dgst.Hex()[:12])
		return nil
	}

	regUrl = distspecBlobUploadURL(registry, strings.Split(repo, ":")[0], skipTLS)

	log.Debugf("New blob upload (POST): %s", regUrl)
	res, err = clientRequest(http.MethodPost, regUrl, username, password, nil, nil)
	if err != nil {
		log.Errorf("post unable to check subject:%s, err:%s", subject, err)
		return err
	}
	log.Debugf("HTTP response status:%v headers:%v", res.Status, dumpHTTPHeaders(res.Header))

	loc, err := res.Location()
	if err != nil {
		log.Errorf("unable get upload location url:%s, err:%s", regUrl, err)
		return err
	}

	log.Debugf("Finish blob upload (PUT): %s", regUrl)
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

// getArtifact to a registry/repo for this subject
func getArtifact(path, mtype, aUrl, username, password string, skipTLS bool) error {
	durl, err := parseDistSpecUrl(aUrl)
	if err != nil {
		log.Warnf("unable to parse url: %s", aUrl)
		return err
	}

	registry := durl.Host
	repo := durl.Path
	subjectTag := durl.Tag
	subject := distspecManifestURL(registry, repo, subjectTag, skipTLS)

	// check subject exists
	res, err := clientRequest(http.MethodHead, subject, username, password, nil, nil)
	if err != nil {
		log.Errorf("unable to check subject:%s, err:%s", subject, err)
		return err
	}
	if res == nil {
		log.Errorf("unable to check subject:%s", subject)
		return errors.Errorf("unable to check subject:%s", subject)
	}

	slen := res.ContentLength
	smtype := res.Header.Get("Content-Type")
	sdgst, err := godigest.Parse(res.Header.Get("Docker-Content-Digest"))
	if slen < 0 || smtype == "" || sdgst == "" || err != nil {
		log.Errorf("unable to get descriptor details for subject:%s", subject)
		return errors.Errorf("unable to get descriptor details for subject:%s", subject)
	}

	// download the artifact
	refsURL := distspecReferrerURL(registry, repo, sdgst.String(), artifactTypeSPDX, skipTLS)
	res, err = clientRequest(http.MethodGet, refsURL, username, password, map[string]string{"Accept": ispec.MediaTypeImageIndex}, nil)
	if err != nil {
		log.Errorf("unable to get references for %s, err:%s", sdgst.String(), err)
		return err
	}
	defer res.Body.Close()

	var index ispec.Index
	err = json.NewDecoder(res.Body).Decode(&index)
	if err != nil {
		return err
	}

	// we expect only one per artifactType
	ref := index.Manifests[0].Digest

	manifestURL := distspecManifestURL(registry, repo, ref.String(), skipTLS)
	res, err = clientRequest(http.MethodGet, manifestURL, username, password, nil, nil)
	if err != nil {
		log.Errorf("unable to get references for %s, err:%s", sdgst.String(), err)
		return err
	}
	defer res.Body.Close()

	var manifest ispec.Manifest
	err = json.NewDecoder(res.Body).Decode(&manifest)
	if err != nil {
		return err
	}

	if (manifest.Config.MediaType != ispec.DescriptorEmptyJSON.MediaType) ||
		(manifest.Config.Digest != ispec.DescriptorEmptyJSON.Digest) {
		log.Errorf("invalid artifact descriptor for %s", sdgst.String())
		return errors.Errorf("invalid artifact descriptor for %s", sdgst.String())
	}

	// create a tempfile
	fh, err := os.CreateTemp(path, "*.json")
	if err != nil {
		log.Errorf("unable to open file:%s, err:%s", path, err)
		return err
	}
	defer fh.Close()

	// skipping additional OCI "artifact" checks
	if err := downloadBlob(registry, repo, path, username, password, fh, manifest.Layers[0].Size, &manifest.Layers[0].Digest, skipTLS); err != nil {
		log.Errorf("unable to download file:%s, err:%s", path, err)
		return err
	}

	log.Infof("Copying artifact %s:%s done", path, mtype)

	return nil
}

func downloadBlob(registry, repo, path, username, password string, writer io.Writer, size int64, dgst *godigest.Digest, skipTLS bool) error {
	// upload with POST, PUT sequence
	blobURL := distspecBlobURL(registry, repo, dgst.String(), skipTLS)

	/* assume the image is present? */
	log.Debugf("Get blob (GET): %s", blobURL)
	res, err := clientRequest(http.MethodGet, blobURL, username, password, nil, nil)
	if err != nil {
		log.Errorf("unable to get blob:%s, err:%s", blobURL, err)
		return err
	}
	defer res.Body.Close()

	_, err = io.Copy(writer, res.Body)
	if err != nil {
		log.Errorf("unable to copy blob:%s, err:%s", blobURL, err)
		return err
	}

	return nil
}

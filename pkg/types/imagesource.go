package types

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

// dockerishUrl represents a URL that looks like docker://image:tag; as of go
// 1.12.9 these are no longer parsed correctly via the url.Parse() function,
// since it complains about :tag not being a valid int (i.e. port number).
type dockerishUrl struct {
	Scheme string
	Host   string
	Tag    string
	Path   string
}

func NewDockerishUrl(thing string) (dockerishUrl, error) {
	parts := strings.SplitN(thing, "://", 2)

	if len(parts) < 2 {
		return dockerishUrl{Path: thing}, nil
	}

	url := dockerishUrl{Scheme: parts[0]}
	pathSplit := strings.SplitN(parts[1], "/", 2)

	url.Host = pathSplit[0]
	if len(pathSplit) == 2 {
		url.Path = "/" + pathSplit[1]
	}

	tagSplit := strings.SplitN(url.Host, ":", 2)
	if len(tagSplit) == 2 {
		url.Tag = tagSplit[1]
	}

	return url, nil
}

type ImageSource struct {
	Type     string `yaml:"type"`
	Url      string `yaml:"url"`
	Tag      string `yaml:"tag"`
	Insecure bool   `yaml:"insecure"`
}

func NewImageSource(containersImageString string) (*ImageSource, error) {
	ret := &ImageSource{}
	if strings.HasPrefix(containersImageString, "oci:") {
		ret.Type = OCILayer
		ret.Url = containersImageString[len("oci:"):]
		return ret, nil
	}

	url, err := NewDockerishUrl(containersImageString)
	if err != nil {
		return nil, err
	}

	switch url.Scheme {
	case "docker":
		ret.Type = DockerLayer
		ret.Url = containersImageString
	default:
		return nil, errors.Errorf("unknown image source type: %s", containersImageString)
	}

	return ret, nil
}

// Returns a URL that can be passed to github.com/containers/image handling
// code.
func (is *ImageSource) ContainersImageURL() (string, error) {
	switch is.Type {
	case DockerLayer:
		return is.Url, nil
	case OCILayer:
		return fmt.Sprintf("oci:%s", is.Url), nil
	default:
		return "", errors.Errorf("can't get containers/image url for source type: %s", is.Type)
	}
}

func (is *ImageSource) ParseTag() (string, error) {
	switch is.Type {
	case BuiltLayer:
		return is.Tag, nil
	case DockerLayer:
		url, err := NewDockerishUrl(is.Url)
		if err != nil {
			return "", err
		}

		if url.Path != "" {
			return path.Base(strings.Split(url.Path, ":")[0]), nil
		}

		// skopeo allows docker://centos:latest or
		// docker://docker.io/centos:latest; if we don't have a
		// url path, let's use the host as the image tag
		return strings.Split(url.Host, ":")[0], nil
	case OCILayer:
		pieces := strings.SplitN(is.Url, ":", 2)
		if len(pieces) != 2 {
			return "", errors.Errorf("bad OCI tag: %s", is.Type)
		}

		return pieces[1], nil
	default:
		return "", errors.Errorf("unsupported type: %s", is.Type)
	}
}

var (
	imageSourceFields []string
)

func init() {
	imageSourceFields = []string{}
	imageSourceType := reflect.TypeOf(ImageSource{})
	for i := 0; i < imageSourceType.NumField(); i++ {
		tag := imageSourceType.Field(i).Tag.Get("yaml")
		imageSourceFields = append(imageSourceFields, tag)
	}
}

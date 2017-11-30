package stacker

import (
	"fmt"
	"os"
	"os/exec"
	"path"
)

func GetBaseLayer(c StackerConfig, name string, l *Layer) error {
	switch(l.From.Type) {
	case BuiltType:
		/* nothing to do assuming layers are imported in dependency order */
		return nil
	case TarType:
		return getTar(c, name, l)
	case OCIType:
		fallthrough
	case DockerType:
		return fmt.Errorf("not implemented")
	default:
		return fmt.Errorf("unknown layer type: %v", l.From.Type)
	}
}

func getTar(c StackerConfig, name string, l *Layer) error {
	tar, err := download(path.Join(c.StackerDir, "layer-bases"), l.From.Url)
	if err != nil {
		return err
	}

	layerPath := path.Join(c.RootFSDir, name)
	if err := os.MkdirAll(layerPath, 0755); err != nil {
		return err
	}

	output, err := exec.Command("tar", "xf", tar, "-C", layerPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %s: %s", err, string(output))
	}

	return nil
}

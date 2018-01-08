package main

import (
	"fmt"
	"io"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	"github.com/urfave/cli"
)

var unladeCmd = cli.Command{
	Name:   "unlade",
	Usage:  "unpacks an OCI image to a directory",
	Action: doUnlade,
	Flags:  []cli.Flag{},
}

func doUnlade(ctx *cli.Context) error {
	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}

	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}

	tags, err := oci.ListTags()
	if err != nil {
		return err
	}

	imported := map[string]bool{}

	for _, tag := range tags {
		blobs, err := oci.LayersForTag(tag)
		if err != nil {
			return err
		}

		for _, b := range blobs {
			defer b.Close()
		}

		for _, b := range blobs {
			reader, ok := b.Data.(io.ReadCloser)
			if !ok {
				return fmt.Errorf("couldn't cast blob data to reader")
			}

			defer reader.Close()

			d := string(b.Digest)
			_, ok = imported[d]
			if ok {
				continue
			} else {
				imported[d] = true
			}

			diffType, err := stacker.MediaTypeToDiffStrategy(b.MediaType)
			if err != nil {
				return err
			}

			err = s.Undiff(diffType, tag, reader)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

package main

import (
	"os"
	"regexp"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func validateBuildFailureFlags(ctx *cli.Context) error {
	if ctx.Bool("shell-fail") {
		askedFor := ctx.String("on-run-failure")
		if askedFor != "" && askedFor != "/bin/sh" {
			return errors.Errorf("--shell-fail is incompatible with --on-run-failure=%s", askedFor)
		}
		err := ctx.Set("on-run-failure", "/bin/sh")
		if err != nil {
			return err
		}
	}

	return nil
}

func validateLayerTypeFlags(ctx *cli.Context) error {
	layerTypes := ctx.StringSlice("layer-type")
	if len(layerTypes) == 0 {
		return errors.Errorf("must specify at least one output --layer-type")
	}

	for _, layerType := range layerTypes {
		switch layerType {
		case "tar":
			break
		case "squashfs":
			break
		default:
			return errors.Errorf("unknown layer type: %s", layerType)
		}
	}

	return nil
}

func validateFileSearchFlags(ctx *cli.Context) error {

	// Use the current working directory if base search directory is "."
	if ctx.String("search-dir") == "." {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		err = ctx.Set("search-dir", wd)
		if err != nil {
			return err
		}
	}

	// Ensure the base search directory exists
	if _, err := os.Lstat(ctx.String("search-dir")); err != nil {
		return err
	}

	// Ensure the stacker-file-pattern variable compiles as a regex
	if _, err := regexp.Compile(ctx.String("stacker-file-pattern")); err != nil {
		return err
	}

	return nil
}

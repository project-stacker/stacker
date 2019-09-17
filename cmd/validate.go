package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/urfave/cli"
)

func validateBuildFailureFlags(ctx *cli.Context) error {
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

func validateLayerTypeFlags(ctx *cli.Context) error {

	switch ctx.String("layer-type") {
	case "tar":
		break
	case "squashfs":
		fmt.Println("squashfs support is experimental")
		break
	default:
		return fmt.Errorf("unknown layer type: %s", ctx.String("layer-type"))
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

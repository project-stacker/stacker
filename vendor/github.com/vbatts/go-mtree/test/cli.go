package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	flag.Parse()

	failed := 0
	for _, arg := range flag.Args() {
		cmd := exec.Command("bash", arg)
		if os.Getenv("TMPDIR") != "" {
			cmd.Env = append(cmd.Env, "TMPDIR="+os.Getenv("TMPDIR"))
		}
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "FAILED: %s\n", arg)
		}
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d FAILED tests\n", failed)
		os.Exit(1)
	}
}

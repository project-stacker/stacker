package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/udhos/equalfile"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("usage: equal file1 file2 [...fileN]\n")
		os.Exit(2)
	}

	if compareFiles(os.Args[1:]) {
		fmt.Println("equal: files match")
		return // cleaner than os.Exit(0)
	}

	fmt.Println("equal: files differ")
	os.Exit(1)
}

func compareFiles(files []string) bool {

	options := equalfile.Options{}

	if str := os.Getenv("DEBUG"); str != "" {
		options.Debug = true
	}

	var cmp *equalfile.Cmp

	if len(files) > 2 {
		cmp = equalfile.NewMultiple(nil, options, sha256.New(), true)
	} else {
		cmp = equalfile.New(nil, options)
	}

	match := true

	for i := 0; i < len(files)-1; i++ {
		p0 := files[i]
		for _, p := range files[i+1:] {
			equal, err := cmp.CompareFile(p0, p)
			if err != nil {
				fmt.Printf("equal(%s,%s): error: %v\n", p0, p, err)
				match = false
				continue
			}
			if !equal {
				if options.Debug {
					fmt.Printf("equal(%s,%s): files differ\n", p0, p)
				}
				match = false
				continue
			}
			if options.Debug {
				fmt.Printf("equal(%s,%s): files match\n", p0, p)
			}
		}
	}

	return match
}

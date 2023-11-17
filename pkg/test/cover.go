package test

import "os"

const CoverageBindPath = "/stacker/.coverage"

func IsCoverageEnabled() bool {
	_, ok := os.LookupEnv("GOCOVERDIR")
	return ok
}

func GetCoverageDir() string {
	val, ok := os.LookupEnv("GOCOVERDIR")
	if ok {
		return val
	}

	return ""
}

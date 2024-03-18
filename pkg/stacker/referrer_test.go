package stacker

import (
	"reflect"
	"testing"
)

func TestDistspecURLParsing(t *testing.T) {
	cases := map[string]*distspecUrl{
		"docker://alpine:latest":                      &distspecUrl{Scheme: "docker", Host: "docker.io", Tag: "latest", Path: "/library/alpine"},
		"docker://localhost:8080/alpine:latest":       &distspecUrl{Scheme: "docker", Host: "localhost:8080", Tag: "latest", Path: "/alpine"},
		"docker://localhost:8080/a/b/c/alpine:latest": &distspecUrl{Scheme: "docker", Host: "localhost:8080", Tag: "latest", Path: "/a/b/c/alpine"},
		"docker://alpine":                             &distspecUrl{Scheme: "docker", Host: "docker.io", Tag: "latest", Path: "/alpine"},
	}

	for input, expected := range cases {
		result, err := parseDistSpecUrl(input)
		if err != nil {
			t.Fatalf("Unable to parse url %s: %s", input, err)
		}

		if !reflect.DeepEqual(*expected, result) {
			t.Fatalf("%s: Incorrect result expected != found: %v != %v",
				input, *expected, result)
		}
	}
}

package stacker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stackerbuild.io/stacker/pkg/types"
)

func TestConverterConvertCommandErrors(t *testing.T) {
	tests := []struct {
		name string
		c    *Converter
		cmd  *Command
	}{
		{
			name: "unsupported from",
			c:    NewConverter(&ConvertArgs{}),
			cmd:  &Command{Cmd: "from", Value: []string{"busybox", "extra"}},
		},
		{
			name: "invalid env",
			c:    NewConverter(&ConvertArgs{}),
			cmd:  &Command{Cmd: "env", Original: "ENV FOO", Value: []string{"FOO"}},
		},
		{
			name: "invalid arg",
			c:    NewConverter(&ConvertArgs{}),
			cmd:  &Command{Cmd: "arg", Value: []string{"FOO=bar=baz"}},
		},
		{
			name: "invalid copy uid",
			c: func() *Converter {
				c := NewConverter(&ConvertArgs{})
				c.currLayer = "layer"
				c.output[c.currLayer] = &types.Layer{}
				return c
			}(),
			cmd: &Command{
				Cmd:      "copy",
				Original: "COPY --chown=bad src /dest",
				Flags:    []string{"--chown=bad"},
				Value:    []string{"src", "/dest"},
			},
		},
		{
			name: "invalid copy gid",
			c: func() *Converter {
				c := NewConverter(&ConvertArgs{})
				c.currLayer = "layer"
				c.output[c.currLayer] = &types.Layer{}
				return c
			}(),
			cmd: &Command{
				Cmd:      "copy",
				Original: "COPY --chown=1:bad src /dest",
				Flags:    []string{"--chown=1:bad"},
				Value:    []string{"src", "/dest"},
			},
		},
		{
			name: "unknown command",
			c: func() *Converter {
				c := NewConverter(&ConvertArgs{})
				c.currLayer = "layer"
				c.output[c.currLayer] = &types.Layer{}
				return c
			}(),
			cmd: &Command{Cmd: "wat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.convertCommand(tt.cmd); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestConverterConvertErrors(t *testing.T) {
	t.Run("missing input", func(t *testing.T) {
		dir := t.TempDir()
		c := NewConverter(&ConvertArgs{
			InputFile:      filepath.Join(dir, "missing.Dockerfile"),
			OutputFile:     filepath.Join(dir, "stacker.yaml"),
			SubstituteFile: filepath.Join(dir, "stacker.subs"),
		})

		if err := c.Convert(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("parse failure", func(t *testing.T) {
		dir := t.TempDir()
		label := strings.Repeat("a", 70_000)
		input := filepath.Join(dir, "Dockerfile")
		if err := os.WriteFile(input, []byte("FROM image\nLABEL test="+label+"\n"), 0644); err != nil {
			t.Fatalf("write dockerfile: %v", err)
		}

		c := NewConverter(&ConvertArgs{
			InputFile:      input,
			OutputFile:     filepath.Join(dir, "stacker.yaml"),
			SubstituteFile: filepath.Join(dir, "stacker.subs"),
		})

		if err := c.Convert(); err == nil {
			t.Fatal("expected error")
		}
	})
}

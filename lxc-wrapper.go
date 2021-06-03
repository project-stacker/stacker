package stacker

import "embed"

//go:embed lxc-wrapper/lxc-wrapper
var embeddedFS embed.FS

//go:build !skipembed

package main

import "embed"

//go:embed lxc-wrapper/lxc-wrapper
var embeddedFS embed.FS

const hasEmbedded = true

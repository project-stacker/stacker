//go:build !skipembed && static_build

package main

import "embed"

//go:embed lxc-wrapper/lxc-wrapper-static
var embeddedFS embed.FS

const hasEmbedded = true
const embeddedWrapperPath = "lxc-wrapper/lxc-wrapper-static"

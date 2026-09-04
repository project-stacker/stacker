//go:build !skipembed && !static_build

package main

import "embed"

//go:embed lxc-wrapper/lxc-wrapper-host
var embeddedFS embed.FS

const hasEmbedded = true
const embeddedWrapperPath = "lxc-wrapper/lxc-wrapper-host"

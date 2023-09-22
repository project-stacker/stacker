//go:build skipembed

package main

import "embed"

var embeddedFS embed.FS

const hasEmbedded = true

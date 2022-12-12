//go:build static_build
// +build static_build

package squashfs

// cryptsetup's pkgconfig is broken (it does not set Requires.private or
// Libs.private at all), so we do the LDLIBS for it by hand.

// #cgo LDFLAGS: -lcryptsetup -lcrypto -lssl -lblkid -luuid -ljson-c -lpthread -ldl
import "C"

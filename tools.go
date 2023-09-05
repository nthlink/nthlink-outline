//go:build tools

package tools

import (
	// Implicitly requires this because we'll need it while building to mobile.
	_ "golang.org/x/mobile/bind"
)

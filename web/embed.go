// Package web exposes the compiled React SPA as an embedded filesystem.
// The dist/ directory is populated by running: make ui-build
// Run "make ui-build" before "go build" to populate this directory.
package web

import "embed"

// Files contains the compiled frontend built by Vite into web/dist/.
//
//go:generate make ui-build
//go:embed dist
var Files embed.FS

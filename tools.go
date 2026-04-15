//go:build tools
// +build tools

package tools

import (
	_ "github.com/a-h/templ/cmd/templ"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/swaggo/http-swagger"
	_ "github.com/swaggo/swag/cmd/swag"
	_ "github.com/vektra/mockery/v2"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "gotest.tools/gotestsum"
)

// Package openapi exposes the generated, versioned API contract compiled into the binary.
package openapi

import _ "embed"

//go:embed swagger.json
var JSON []byte

//go:embed swagger.yaml
var YAML []byte

// Package apispec guards the OpenAPI spec of docs/api/openapi against the
// routes the server really registers (ADR 0012, decision 3).
//
// It has no production code: its tests parse the Go sources of every
// routes.go and of cmd/server/main.go with go/parser and go/ast, read the spec
// with go.yaml.in/yaml/v3 following its $ref, and compare the "METHOD /path"
// set and the role (x-rol) that the middleware chain of each route enforces.
// No database is needed, so it runs in a plain `go test ./...`.
package apispec

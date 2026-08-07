// Package web embeds the UI. index.html is a placeholder until the UI agent
// delivers the real page per docs/ui-spec.md.
package web

import _ "embed"

//go:embed index.html
var IndexHTML []byte

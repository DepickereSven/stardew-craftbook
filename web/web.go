// Package web embeds the UI: a single self-contained page covering the
// recipes view (docs/ui-spec.md) and the items view (docs/ui-spec-items.md).
package web

import _ "embed"

//go:embed index.html
var IndexHTML []byte

// Package templates provides embedded issue templates.
package templates

import _ "embed"

// MinimalTemplate is the starter issue frontmatter.
//
//go:embed draft.issue.md
var MinimalTemplate string

// ElaborateTemplate is the full issue template reference.
//
//go:embed template.issue.md
var ElaborateTemplate string

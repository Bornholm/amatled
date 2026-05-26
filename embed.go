package amatled

import "embed"

// WebFS contient les assets frontend embarqués (HTML + bundle JS).
//
//go:embed web/index.html web/dist
var WebFS embed.FS

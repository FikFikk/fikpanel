package web

import "embed"

// DistFS menampung seluruh aset statis frontend Svelte yang dikompilasi ke direktori dist
//
//go:embed all:dist
var DistFS embed.FS

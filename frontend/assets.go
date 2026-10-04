package frontend

import "embed"

// Build the frontend before compiling the desktop entry point.
//
//go:embed all:dist
var Assets embed.FS

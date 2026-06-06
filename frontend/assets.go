//go:build !dev

package frontend

import "embed"

// `all:` prefix is required so Vite-generated chunks whose names start with
// `_` (e.g. `_Uint8Array-*.js` from mermaid's dynamic imports) are included.
// Without it, Go's embed.FS silently drops files beginning with `_` or `.`,
// and the SPA hangs on the missing dynamic import.
//go:embed all:dist
var assets embed.FS

func Assets() embed.FS {
	return assets
}

// Package migrations meng-embed file SQL goose ke dalam binary.
// Buat file baru dengan: make migrate-new name=create_xxx
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

package migrations

import (
	"embed"
	"io/fs"
)

// files contains ordered SQLite schema migrations.
//
//go:embed *.sql
var files embed.FS

func Files() fs.FS {
	return files
}

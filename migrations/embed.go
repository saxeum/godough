// Package migrations carries the .sql files into the migrator binary.
//
// Embed, not COPY into the image: the image has no shell and no /migrations, so
// a directory the Job is pointed at can only come from a second copy that drifts
// from this one. The binary is the source of truth, and the Job runs the digest
// that is deployed.
package migrations

import "embed"

// FS holds every migration, keyed by bare filename.
//
//go:embed *.sql
var FS embed.FS

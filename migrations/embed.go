package migrations

import "embed"

// FS 内嵌全部 SQL 迁移，交给 goose 执行。
//
//go:embed *.sql
var FS embed.FS

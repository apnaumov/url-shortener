// Package urlshortener встраивает SQL-миграции в бинарник.
package urlshortener

import "embed"

// MigrationsFS содержит SQL-файлы миграций из каталога migrations.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

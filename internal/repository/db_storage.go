package repository

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DB *sql.DB

func InitFromConnectionString(connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	DB = db

	return nil
}

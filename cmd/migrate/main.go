package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	db, err := sql.Open("pgx", url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "set dialect: %v\n", err)
		os.Exit(1)
	}

	if err := goose.Run(command, db, "."); err != nil {
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", command, err)
		os.Exit(1)
	}
}

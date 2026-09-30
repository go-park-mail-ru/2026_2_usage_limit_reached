package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "pgx5://patreon:patreon@localhost:8001/patreon?sslmode=disable" // sslmode=disable только в локальной разработке
	}

	m, err := migrate.New(
		"file://db/migrations",
		dsn,
	)
	if err != nil {
		log.Fatalf("migration initialization error : %v\n", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migration execution error: %v\n", err)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no new migrations.")
	} else {
		fmt.Println("migrations successfully applied")
	}
}

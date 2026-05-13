package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/cycloidio/terracost/mysql"
)

func main() {
	dsn := os.Getenv("TERRACOST_DSN")
	if dsn == "" {
		dsn = "root:terracost@tcp(172.44.0.2:3306)/terracost_test?multiStatements=true"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	if err := mysql.Migrate(context.Background(), db, "_migrations"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Migrated successfully!")
}

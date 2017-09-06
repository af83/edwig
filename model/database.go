package model

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/af83/edwig/config"
	"github.com/af83/edwig/logger"
	"github.com/rubenv/sql-migrate"
	"gopkg.in/gorp.v1"

	_ "github.com/lib/pq"
)

var Database *gorp.DbMap

func InitDB(config config.DatabaseConfig) *gorp.DbMap {
	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable",
		config.User,
		config.Password,
		config.Name,
	)
	db, err := sql.Open("postgres", dbinfo)
	if err != nil {
		logger.Log.Panicf("Error while connecting to the database:\n%v", err)
	}
	logger.Log.Debugf("Connected to Database %s", config.Name)
	// construct a gorp DbMap
	database := &gorp.DbMap{Db: db, Dialect: gorp.PostgresDialect{}}
	database.AddTableWithName(DatabaseReferential{}, "referentials")
	database.AddTableWithName(DatabasePartner{}, "partners")

	return database
}

func CloseDB(database *gorp.DbMap) {
	database.Db.Close()
}

func InitTestDb(t *testing.T) {
	config.SetEnvironment("test")
	// Load configuration
	err := config.LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	config.Config.ApiKey = ""
	// Initialize Database
	Database = InitDB(config.Config.DB)

	_, err = Database.Exec("TRUNCATE referentials, partners, lines, operators, stop_areas, stop_visits;")
	if err != nil {
		t.Fatal(err)
	}

	_, err = Database.Exec("BEGIN;")
	if err != nil {
		t.Fatal(err)
	}
}

func CleanTestDb(t *testing.T) {
	_, err := Database.Exec("ROLLBACK;")
	if err != nil {
		t.Fatal(err)
	}
	CloseDB(Database)
}

func ApplyMigrations(operation, path string, database *sql.DB) error {
	migrations := &migrate.FileMigrationSource{
		Dir: path,
	}

	var n int
	var err error
	switch operation {
	case "up":
		n, err = migrate.Exec(database, "postgres", migrations, migrate.Up)
	case "down":
		n, err = migrate.Exec(database, "postgres", migrations, migrate.Down)
	}
	if err != nil {
		return err
	}
	logger.Log.Debugf("Applied %d migrations\n", n)

	return nil
}

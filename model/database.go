package model

import (
	"database/sql"
	"fmt"

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

	return database
}

func CloseDB(database *gorp.DbMap) {
	database.Db.Close()
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
package database

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"vigilantgo/migrations"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

var migrationFiles embed.FS

func RunMigrations(cmd string, version int) error {
	slog.Info(fmt.Sprintf("Running migration: %s", cmd))
	connStr := os.Getenv("MIGRATOR_DB_URL")
	if connStr == "" {
		return errors.New("MIGRATOR_DB_URL is required")
	}

	d, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, connStr)
	if err != nil {
		return err
	}
	defer m.Close()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return err
		}
		slog.Info("Applied all pending migrations")
	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			return err
		}
		slog.Info("Rolled back last migration")
		return nil
	case "force":
		if version == 0 {
			log.Fatal("Need version for force command")
			return err
		}
		if err := m.Force(version); err != nil && err != migrate.ErrNoChange {
			return err
		}
		slog.Info(fmt.Sprintf("Forced version to %d", version))
	case "version":
		v, dirty, err := m.Version()
		if err != nil && err != migrate.ErrNilVersion {
			return err
		}
		slog.Info(fmt.Sprintf("Current version: %d, dirty: %v", v, dirty))
		return nil
	default:
		slog.Error("Unknown command", "cmd", cmd)
	}
	return nil
}

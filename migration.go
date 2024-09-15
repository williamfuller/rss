package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

func migrate(d *sql.DB) error {
	root := "migrations"
	migrations, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("readDir migrations: %w", err)
	}

	tx, err := d.BeginTx(context.Background(), &sql.TxOptions{Isolation: 6})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	var fileNames []string
	for _, migration := range migrations {
		fileNames = append(fileNames, migration.Name())
	}
	slices.Sort(fileNames)

	for _, fileName := range fileNames {
		file, err := os.ReadFile(filepath.Join(root, fileName))
		if err != nil {
			err2 := tx.Rollback()
			return fmt.Errorf("read file %s: %w", fileName, errors.Join(err, err2))
		}

		err = tx.QueryRow("SELECT name FROM migrations WHERE name = $1", fileName).Scan(&fileName)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			err2 := tx.Rollback()
			return fmt.Errorf("SELECT %s: %w", fileName, errors.Join(err, err2))
		} else if err == nil {
			continue
		}

		_, err = tx.Exec(string(file))
		if err != nil {
			err2 := tx.Rollback()
			return fmt.Errorf("exec %s: %w", fileName, errors.Join(err, err2))
		}

		_, err = tx.Exec("INSERT INTO migrations (name) VALUES ($1)", fileName)
		if err != nil {
			err2 := tx.Rollback()
			return fmt.Errorf("INSERT %s: %w", fileName, errors.Join(err, err2))
		}
	}

	err = tx.Commit()
	if err != nil {
		err2 := tx.Rollback()
		return fmt.Errorf("commit transaction: %w", errors.Join(err, err2))
	}

	return nil
}

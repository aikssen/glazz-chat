package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/aikssen/glazz-chat/apps/api/migrations"
)

type MigrationRunner struct {
	database *sql.DB
	provider *goose.Provider
	hashes   map[int64]string
}

func NewMigrationRunner(databaseURL string) (*MigrationRunner, error) {
	pgxConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse migration database configuration: %w", err)
	}
	database := stdlib.OpenDB(*pgxConfig)
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrations.Files)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	hashes, err := migrationHashes(migrations.Files)
	if err != nil {
		provider.Close()
		database.Close()
		return nil, err
	}
	return &MigrationRunner{database: database, provider: provider, hashes: hashes}, nil
}

func (r *MigrationRunner) Close() error {
	providerErr := r.provider.Close()
	databaseErr := r.database.Close()
	return errors.Join(providerErr, databaseErr)
}

func (r *MigrationRunner) Up(ctx context.Context) error {
	if err := r.verifyApplied(ctx); err != nil {
		return err
	}
	if _, err := r.provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if err := r.recordApplied(ctx); err != nil {
		return err
	}
	return nil
}

func (r *MigrationRunner) Down(ctx context.Context) error {
	if err := r.verifyApplied(ctx); err != nil {
		return err
	}
	if _, err := r.provider.Down(ctx); err != nil {
		return fmt.Errorf("roll back migration: %w", err)
	}
	return r.recordApplied(ctx)
}

func (r *MigrationRunner) Reset(ctx context.Context) error {
	if err := r.verifyApplied(ctx); err != nil {
		return err
	}
	if _, err := r.provider.DownTo(ctx, 0); err != nil {
		return fmt.Errorf("reset migrations: %w", err)
	}
	if _, err := r.provider.Up(ctx); err != nil {
		return fmt.Errorf("reapply migrations: %w", err)
	}
	return r.recordApplied(ctx)
}

func (r *MigrationRunner) Validate(ctx context.Context) error {
	if _, err := r.provider.Status(ctx); err != nil {
		return fmt.Errorf("read migration status: %w", err)
	}
	return r.verifyApplied(ctx)
}

func (r *MigrationRunner) verifyApplied(ctx context.Context) error {
	var exists bool
	if err := r.database.QueryRowContext(
		ctx,
		`SELECT to_regclass('public.glazz_migration_checksums') IS NOT NULL`,
	).Scan(&exists); err != nil {
		return fmt.Errorf("inspect migration checksum table: %w", err)
	}
	if !exists {
		return nil
	}

	rows, err := r.database.QueryContext(ctx, `SELECT version, checksum FROM glazz_migration_checksums`)
	if err != nil {
		return fmt.Errorf("read migration checksums: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version int64
		var actual string
		if err := rows.Scan(&version, &actual); err != nil {
			return fmt.Errorf("scan migration checksum: %w", err)
		}
		expected, ok := r.hashes[version]
		if !ok || expected != actual {
			return fmt.Errorf("migration drift detected at version %d", version)
		}
	}
	return rows.Err()
}

func (r *MigrationRunner) recordApplied(ctx context.Context) error {
	var exists bool
	if err := r.database.QueryRowContext(
		ctx,
		`SELECT to_regclass('public.glazz_migration_checksums') IS NOT NULL`,
	).Scan(&exists); err != nil {
		return fmt.Errorf("inspect migration checksum table: %w", err)
	}
	if !exists {
		return nil
	}

	var current int64
	if err := r.database.QueryRowContext(
		ctx,
		`SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied`,
	).Scan(&current); err != nil {
		return fmt.Errorf("read current migration version: %w", err)
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin checksum transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM glazz_migration_checksums WHERE version > $1`, current); err != nil {
		return fmt.Errorf("remove rolled-back checksums: %w", err)
	}
	for version, checksum := range r.hashes {
		if version > current {
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO glazz_migration_checksums (version, checksum)
			 VALUES ($1, $2)
			 ON CONFLICT (version) DO NOTHING`,
			version,
			checksum,
		); err != nil {
			return fmt.Errorf("record migration checksum: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit checksum transaction: %w", err)
	}
	return nil
}

func migrationHashes(files fs.FS) (map[int64]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	result := make(map[int64]string, len(names))
	for _, name := range names {
		prefix := strings.SplitN(filepath.Base(name), "_", 2)[0]
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", name, err)
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", name, err)
		}
		sum := sha256.Sum256(content)
		result[version] = hex.EncodeToString(sum[:])
	}
	return result, nil
}

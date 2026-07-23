package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

type Pool struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, cfg config.Database) (*Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	poolConfig.MaxConns = cfg.MaxConnections
	poolConfig.MinConns = cfg.MinConnections
	poolConfig.MaxConnLifetime = cfg.MaxLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	healthCtx, cancel := context.WithTimeout(ctx, cfg.HealthTimeout)
	defer cancel()
	if err := pool.Ping(healthCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Pool{pool: pool}, nil
}

func (p *Pool) Close() {
	p.pool.Close()
}

func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Pool) Queries() *store.Queries {
	return store.New(p.pool)
}

func (p *Pool) Raw() *pgxpool.Pool {
	return p.pool
}

type TransactionFunc func(queries *store.Queries) error

func (p *Pool) WithinTransaction(ctx context.Context, options pgx.TxOptions, fn TransactionFunc) error {
	tx, err := p.pool.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(store.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (p *Pool) WithAdvisoryLock(
	ctx context.Context,
	key string,
	fn func() error,
) (bool, error) {
	connection, err := p.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire advisory-lock connection: %w", err)
	}
	defer connection.Release()

	var acquired bool
	if err := connection.QueryRow(
		ctx,
		`SELECT pg_try_advisory_lock(hashtextextended($1, 0))`,
		key,
	).Scan(&acquired); err != nil {
		return false, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !acquired {
		return false, nil
	}
	defer func() {
		_, _ = connection.Exec(
			context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock(hashtextextended($1, 0))`,
			key,
		)
	}()
	return true, fn()
}

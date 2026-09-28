package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type key struct{}

// TransactionManager открывает транзакции и кладет их в контекст, откуда их берет giver.
type TransactionManager struct {
	pool     *pgxpool.Pool
	isoLevel pgx.TxIsoLevel
}

// NewTransactionManager создает TransactionManager с уровнем изоляции Read Committed.
func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{
		pool:     pool,
		isoLevel: pgx.ReadCommitted,
	}
}

// Do выполняет fn в транзакции: открывает ее с уровнем изоляции менеджера, кладет в контекст
// и вызывает fn с этим контекстом. fn вернула nil — COMMIT, вернула ошибку или паникнула —
// ROLLBACK и проброс наружу. Если транзакция уже есть в ctx (вложенный Do), fn выполняется в ней.
func (t *TransactionManager) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(key{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: t.isoLevel})
	if err != nil {
		return fmt.Errorf("failed begin transaction: %w", err)
	}

	ctxTransaction := context.WithValue(ctx, key{}, tx)

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			err = fmt.Errorf("commit tx: %w", commitErr)
		}
	}()

	return fn(ctxTransaction)
}

// giver возвращает транзакцию из контекста, если она есть, иначе пул.
func giver(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(key{}).(pgx.Tx); ok {
		return tx
	}

	return pool
}

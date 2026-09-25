package internal

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX представляет интерфейс для работы с базой.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxManager определяет контракт менеджера транзакций.
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// key является типом для ключа контекста, чтобы избежать коллизий.
type txKey struct{}

// txManager хранит ссылку на пулл соединений.
type txManager struct {
	pool *pgxpool.Pool
}

// NewTxManager создает новый экземпляр менеджера транзакций.
func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &txManager{
		pool: pool,
	}
}

// GetExecutor проверяет наличие транзакции в контексте.
func GetExecutor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// Do выполняет переданную функцию fn внутри транзакции.
func (m *txManager) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	// проверка на отсутствие открытых транзакций
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	// новая транзакция
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// закрытие транзакции
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				err = fmt.Errorf("err: %w, rollback err: %v", err, rbErr)
			}
		} else {
			if cmErr := tx.Commit(ctx); cmErr != nil {
				err = fmt.Errorf("commit transaction: %w", cmErr)
			}
		}
	}()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	err = fn(txCtx)

	return err
}

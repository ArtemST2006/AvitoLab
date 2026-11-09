package handlers

import (
	"context"
)

// TxManager предоставляет интерфейс для работы с транзакциями
//
// TODO: перенестив сервсный слой
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

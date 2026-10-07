// Package handlers реализует HTTP API Trip Service по контракту из contracts/openapi.
package handlers

import (
	"errors"
	"log/slog"
	"time"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
	"github.com/ArtemST2006/AvitoLab/internal/storage"
)

// ErrAlreadyComplited доменный сентинел на уже завершённый трип
var ErrAlreadyComplited = errors.New("trip already complited")

// Server реализует все эндпоинты контракта.
type Server struct {
	api.Unimplemented

	log       *slog.Logger
	repo      *storage.Repository
	txManager TxManager

	queryTimeout time.Duration
}

var _ api.ServerInterface = (*Server)(nil)

// NewServer создает Server.
func NewServer(log *slog.Logger, repo *storage.Repository, txManager TxManager, queryTimeout time.Duration) *Server {
	return &Server{
		log:          log,
		repo:         repo,
		txManager:    txManager,
		queryTimeout: queryTimeout,
	}
}

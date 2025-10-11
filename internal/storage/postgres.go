package storage

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

type PostegresStorage struct {
	db     *sql.DB
	logger *zap.Logger
}

type StorageEvent struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	Properties map[string]interface{} `json:"properties"`
}

// WindowMetrics représente les métriques d'une fenêtre
type WindowMetrics struct {
	StartTime      time.Time
	EndTime        time.Time
	TotalEvents    int64
	EventTypes     map[string]int64
	UniqueUsers    int64
	UniqueSessions int64
	AllMetrics     map[string]interface{}
}

func NewPostgresStorage(connStr string, maxConns int, logger *zap.Logger) (*PostegresStorage, error) {

}

package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	//ouvrire cnx
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	//configuration du pool
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns / 2)
	db.SetConnMaxLifetime(5 * time.Minute)

	//tester la cnx
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("cant connect to the db : %w", err)
	}
	logger.Info("Connect to PostgreSQL",
		zap.String("connStr", redactConnStr(connStr)),
		zap.Int("MaxConns", maxConns))

	return &PostegresStorage{
		db:     db,
		logger: logger,
	}, nil

}

// redactConnStr pour cacher le passwrd pour les logs
func redactConnStr(connStr string) string {
	return "postgres://user:***@host:5432/dbname"
}

func (ps *PostegresStorage) InsertEvent(ctx context.Context, event StorageEvent) error {
	query := `
	INSERT INTO events (id, time, type, user_id, session_id, properties)
	VALUES ($1, $2, $3, $4, $5, $6)
	`
	propsJSON, _ := json.Marshal(event.Properties)

	_, err := ps.db.ExecContext(ctx, query,
		event.ID,
		event.Timestamp,
		event.Type,
		event.UserID,
		event.SessionID,
		propsJSON,
	)

	if err != nil {
		ps.logger.Error("failed to insert event",
			zap.String("event_id", event.ID),
			zap.Error(err))
		return err
	}

	return nil
}

// insere plusieurs evenmentsen batch
func (ps *PostegresStorage) InsertEventsBatch(ctx context.Context, events []StorageEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := ps.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO events (id, time, type, user_id, session_id, properties)
		VALUES ($1, $2, $3, $4, $5, $6)`)

	if err != nil {
		return fmt.Errorf("failed to prepare statement : %w", err)

	}
	defer stmt.Close()

	for _, event := range events {
		propsJSON, _ := json.Marshal(event.Properties)

		_, err := stmt.ExecContext(ctx,
			event.ID,
			event.Timestamp,
			event.Type,
			event.UserID,
			event.SessionID,
			propsJSON,
		)
		if err != nil {
			ps.logger.Error("failed to insert event in batch",
				zap.String("event_id", event.ID),
				zap.Error(err))
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	ps.logger.Debug("batch inserted",
		zap.Int("count", len(events)),
	)
	return nil
}

// sauvgarede les emtrics dune fenetre fermee
func (ps *PostegresStorage) SaveWindowMetrics(ctx context.Context, metrics WindowMetrics) error {
	query := `
		INSERT INTO closed_windows (start_time, end_time, window_duration, total_events, event_types, unique_users, unique_sessions, metrics_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`

	eventTypesJSON, _ := json.Marshal(metrics.EventTypes)
	metricsDataJSON, _ := json.Marshal(metrics.AllMetrics)

	duration := metrics.EndTime.Sub(metrics.StartTime)
}

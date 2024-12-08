package docket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "docket/proto"
)

// PostgresStore implements Store using github.com/jackc/pgx/v5.
// It is designed to work with pgxpool, similar to River.
type PostgresStore struct {
	pool      *pgxpool.Pool
	tableName string
}

// NewPostgresStore creates a new Postgres-backed store.
func NewPostgresStore(pool *pgxpool.Pool, tableName string) *PostgresStore {
	if tableName == "" {
		tableName = "protograph_store"
	}
	return &PostgresStore{
		pool:      pool,
		tableName: tableName,
	}
}

// InitSchema creates the necessary table if it doesn't exist.
func (s *PostgresStore) InitSchema(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key TEXT PRIMARY KEY,
			value BYTEA,
			error TEXT,
			created_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ,
			metadata JSONB
		);
	`, s.tableName)

	_, err := s.pool.Exec(ctx, query)
	return err
}

func (s *PostgresStore) Get(ctx context.Context, key string) (*pb.PersistenceEntry, error) {
	query := fmt.Sprintf(`
		SELECT value, error, created_at, expires_at, metadata
		FROM %s
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > $2)
	`, s.tableName)

	var value []byte
	var errorStr *string
	var createdAt, expiresAt *time.Time
	var metadataJSON []byte

	err := s.pool.QueryRow(ctx, query, key, time.Now()).Scan(
		&value, &errorStr, &createdAt, &expiresAt, &metadataJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	entry := &pb.PersistenceEntry{}

	// Unmarshal Value (Any)
	if len(value) > 0 {
		anyVal := &anypb.Any{}
		if err := proto.Unmarshal(value, anyVal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal value proto: %w", err)
		}
		entry.Value = anyVal
	}

	if errorStr != nil {
		entry.Error = *errorStr
	}

	if createdAt != nil {
		entry.CreatedAt = timestamppb.New(*createdAt)
	}

	if expiresAt != nil {
		entry.ExpiresAt = timestamppb.New(*expiresAt)
	}

	if len(metadataJSON) > 0 {
		var meta map[string]string
		if err := json.Unmarshal(metadataJSON, &meta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata json: %w", err)
		}
		entry.Metadata = meta
	}

	return entry, nil
}

func (s *PostgresStore) Set(ctx context.Context, key string, entry *pb.PersistenceEntry) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (key, value, error, created_at, expires_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			error = excluded.error,
			created_at = excluded.created_at,
			expires_at = excluded.expires_at,
			metadata = excluded.metadata
	`, s.tableName)

	// Marshal Value
	var valueBytes []byte
	var err error
	if entry.Value != nil {
		valueBytes, err = proto.Marshal(entry.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal value proto: %w", err)
		}
	}

	// Marshal Metadata
	var metaJSON []byte
	if len(entry.Metadata) > 0 {
		metaJSON, err = json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	var createdTime, expiresTime interface{}
	createdTime = time.Now()
	if entry.CreatedAt != nil {
		createdTime = entry.CreatedAt.AsTime()
	}

	if entry.ExpiresAt != nil {
		expiresTime = entry.ExpiresAt.AsTime()
	} else {
		expiresTime = nil
	}

	_, err = s.pool.Exec(ctx, query,
		key,
		valueBytes,
		entry.Error,
		createdTime,
		expiresTime,
		metaJSON,
	)

	return err
}

func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE key = $1", s.tableName)
	_, err := s.pool.Exec(ctx, query, key)
	return err
}

package docket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "docket/proto"
)

// SQLDialect defines the SQL syntax variant.
type SQLDialect string

const (
	DialectSQLite   SQLDialect = "sqlite"
	DialectPostgres SQLDialect = "postgres"
	DialectMySQL    SQLDialect = "mysql"
)

// SQLStore implements Store using database/sql.
// It supports SQLite, Postgres, and MySQL.
type SQLStore struct {
	db        *sql.DB
	tableName string
	dialect   SQLDialect
}

// NewSQLStore creates a new SQL-backed store.
// The user is responsible for opening the *sql.DB with their preferred driver.
func NewSQLStore(db *sql.DB, tableName string, dialect SQLDialect) *SQLStore {
	if tableName == "" {
		tableName = "protograph_store"
	}
	return &SQLStore{
		db:        db,
		tableName: tableName,
		dialect:   dialect,
	}
}

// InitSchema creates the necessary table if it doesn't exist.
// This is a helper for "migration-free" usage.
func (s *SQLStore) InitSchema(ctx context.Context) error {
	blobType := "BLOB"
	textType := "TEXT"
	timestampType := "TIMESTAMP"

	if s.dialect == DialectPostgres {
		blobType = "BYTEA"
	} else if s.dialect == DialectMySQL {
		textType = "TEXT"
		timestampType = "DATETIME"
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key %s PRIMARY KEY,
			value %s,
			error %s,
			created_at %s,
			expires_at %s,
			metadata %s
		);
	`, s.tableName, textType, blobType, textType, timestampType, timestampType, textType)

	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *SQLStore) Get(ctx context.Context, key string) (*pb.PersistenceEntry, error) {
	// Query including expiry check
	// SQLite: ?
	// Postgres: $1, $2
	p1 := "?"
	p2 := "?"
	if s.dialect == DialectPostgres {
		p1 = "$1"
		p2 = "$2"
	}

	query := fmt.Sprintf(`
		SELECT value, error, created_at, expires_at, metadata
		FROM %s
		WHERE key = %s
		  AND (expires_at IS NULL OR expires_at > %s)
	`, s.tableName, p1, p2)

	var value []byte
	var errorStr sql.NullString
	var createdAt, expiresAt sql.NullTime
	var metadataJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, key, time.Now()).Scan(
		&value, &errorStr, &createdAt, &expiresAt, &metadataJSON,
	)

	if err == sql.ErrNoRows {
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

	if errorStr.Valid {
		entry.Error = errorStr.String
	}

	if createdAt.Valid {
		entry.CreatedAt = timestamppb.New(createdAt.Time)
	}

	if expiresAt.Valid {
		entry.ExpiresAt = timestamppb.New(expiresAt.Time)
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var meta map[string]string
		if err := json.Unmarshal([]byte(metadataJSON.String), &meta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata json: %w", err)
		}
		entry.Metadata = meta
	}

	return entry, nil
}

func (s *SQLStore) Set(ctx context.Context, key string, entry *pb.PersistenceEntry) error {
	placeholders := make([]string, 6)
	if s.dialect == DialectPostgres {
		for i := 0; i < 6; i++ {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
	} else {
		for i := 0; i < 6; i++ {
			placeholders[i] = "?"
		}
	}
	phStr := strings.Join(placeholders, ", ")

	// Build upsert query based on dialect
	var query string
	if s.dialect == DialectMySQL {
		query = fmt.Sprintf(`
			INSERT INTO %s (key, value, error, created_at, expires_at, metadata)
			VALUES (%s)
			ON DUPLICATE KEY UPDATE
				value = VALUES(value),
				error = VALUES(error),
				created_at = VALUES(created_at),
				expires_at = VALUES(expires_at),
				metadata = VALUES(metadata)
		`, s.tableName, phStr)
	} else {
		// SQLite and Postgres use ON CONFLICT
		query = fmt.Sprintf(`
			INSERT INTO %s (key, value, error, created_at, expires_at, metadata)
			VALUES (%s)
			ON CONFLICT(key) DO UPDATE SET
				value = excluded.value,
				error = excluded.error,
				created_at = excluded.created_at,
				expires_at = excluded.expires_at,
				metadata = excluded.metadata
		`, s.tableName, phStr)
	}

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

	_, err = s.db.ExecContext(ctx, query,
		key,
		valueBytes,
		entry.Error,
		createdTime,
		expiresTime,
		string(metaJSON),
	)

	return err
}

func (s *SQLStore) Delete(ctx context.Context, key string) error {
	p1 := "?"
	if s.dialect == DialectPostgres {
		p1 = "$1"
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE key = %s", s.tableName, p1)
	_, err := s.db.ExecContext(ctx, query, key)
	return err
}

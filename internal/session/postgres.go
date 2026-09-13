package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgStore Store'ning PostgreSQL implementatsiyasi. Har xabar bitta qator:
// (session_id, seq, message JSONB). schema.Message to'liq JSONB sifatida saqlanadi
// (rol, matn, tool_calls saqlanadi).
type pgStore struct {
	pool *pgxpool.Pool
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS chat_messages (
    id         BIGSERIAL PRIMARY KEY,
    session_id TEXT        NOT NULL,
    message    JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_session ON chat_messages (session_id, id);`

// NewPostgres pool ochadi, ulanishni tekshiradi va jadvalni yaratadi (yo'q bo'lsa).
func NewPostgres(ctx context.Context, dsn string) (Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("chat_messages jadvalini yaratish: %w", err)
	}
	return &pgStore{pool: pool}, nil
}

func (s *pgStore) History(ctx context.Context, id string) ([]*schema.Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT message FROM chat_messages WHERE session_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("tarixni o'qish: %w", err)
	}
	defer rows.Close()

	var out []*schema.Message
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("xabarni skan qilish: %w", err)
		}
		var m schema.Message
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("xabarni tahlil qilish: %w", err)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *pgStore) Append(ctx context.Context, id string, msgs ...*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, m := range msgs {
		raw, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("xabarni JSON'ga o'girish: %w", err)
		}
		batch.Queue(`INSERT INTO chat_messages (session_id, message) VALUES ($1, $2)`, id, raw)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

func (s *pgStore) Reset(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM chat_messages WHERE session_id = $1`, id)
	return err
}

// Close pool'ni yopadi (dastur to'xtaganda).
func (s *pgStore) Close() { s.pool.Close() }

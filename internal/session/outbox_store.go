package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxItem struct {
	ID       int64
	Kind     string
	Payload  []byte
	Attempts int
}

type OutboxStore interface {
	Enqueue(context.Context, string, []byte) error
	Claim(context.Context, int, time.Time) ([]OutboxItem, error)
	Ack(context.Context, int64) error
	Fail(context.Context, int64, time.Time, bool) error
	Pending(context.Context) (pending, dead int, err error)
}

type memoryOutbox struct {
	mu     sync.Mutex
	nextID int64
	items  map[int64]*memoryOutboxItem
}

type memoryOutboxItem struct {
	OutboxItem
	nextAt time.Time
	dead   bool
	claim  bool
}

func NewMemoryOutbox() OutboxStore {
	return &memoryOutbox{items: make(map[int64]*memoryOutboxItem)}
}

func (s *memoryOutbox) Enqueue(ctx context.Context, kind string, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.items[s.nextID] = &memoryOutboxItem{OutboxItem: OutboxItem{ID: s.nextID, Kind: kind, Payload: append([]byte(nil), payload...)}}
	return nil
}

func (s *memoryOutbox) Claim(ctx context.Context, limit int, now time.Time) ([]OutboxItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OutboxItem, 0, limit)
	for _, item := range s.items {
		if len(out) >= limit || item.dead || item.claim || (!item.nextAt.IsZero() && item.nextAt.After(now)) {
			continue
		}
		item.claim = true
		item.Payload = append([]byte(nil), item.Payload...)
		out = append(out, item.OutboxItem)
	}
	return out, nil
}

func (s *memoryOutbox) Ack(_ context.Context, id int64) error {
	s.mu.Lock()
	delete(s.items, id)
	s.mu.Unlock()
	return nil
}

func (s *memoryOutbox) Fail(ctx context.Context, id int64, nextAt time.Time, dead bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if item := s.items[id]; item != nil {
		item.Attempts++
		item.nextAt = nextAt
		item.dead = dead
		item.claim = false
	}
	return nil
}

func (s *memoryOutbox) Pending(_ context.Context) (pending, dead int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.dead {
			dead++
		} else {
			pending++
		}
	}
	return pending, dead, nil
}

// NewPostgresOutbox idempotent jadval yaratadi; shu sababli eski migratorga tegmasdan
// deploy paytida 0002 sxemasini ham xavfsiz qo'llash mumkin.
func NewPostgresOutbox(source any, dsn ...string) (OutboxStore, error) {
	var pool *pgxpool.Pool
	var ctx context.Context
	var err error
	switch value := source.(type) {
	case *pgxpool.Pool:
		pool = value
		ctx = context.Background()
	case context.Context:
		if len(dsn) != 1 {
			return nil, fmt.Errorf("outbox DSN berilmagan")
		}
		ctx = value
		pool, err = pgxpool.New(ctx, dsn[0])
	case string:
		ctx = context.Background()
		pool, err = pgxpool.New(ctx, value)
	default:
		return nil, fmt.Errorf("outbox pool yoki DSN kerak")
	}
	if err != nil {
		return nil, fmt.Errorf("outbox pool: %w", err)
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS agent; CREATE TABLE IF NOT EXISTS agent.outbox (id BIGSERIAL PRIMARY KEY, kind TEXT NOT NULL, payload JSONB NOT NULL, attempts INT NOT NULL DEFAULT 0, next_at TIMESTAMPTZ NOT NULL DEFAULT now(), created_at TIMESTAMPTZ NOT NULL DEFAULT now(), dead BOOLEAN NOT NULL DEFAULT false); CREATE INDEX IF NOT EXISTS outbox_pending_idx ON agent.outbox (dead, next_at)`); err != nil {
		pool.Close()
		return nil, fmt.Errorf("outbox jadvali: %w", err)
	}
	return &postgresOutbox{pool: pool}, nil
}

type postgresOutbox struct{ pool *pgxpool.Pool }

func (s *postgresOutbox) Enqueue(ctx context.Context, kind string, payload []byte) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO agent.outbox(kind,payload) VALUES($1,$2::jsonb)`, kind, payload)
	if err != nil {
		return fmt.Errorf("outbox qo'shish: %w", err)
	}
	return nil
}

func (s *postgresOutbox) Claim(ctx context.Context, limit int, now time.Time) ([]OutboxItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,kind,payload,attempts FROM agent.outbox WHERE NOT dead AND next_at <= $1 ORDER BY id FOR UPDATE SKIP LOCKED LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox olish: %w", err)
	}
	defer rows.Close()
	var out []OutboxItem
	for rows.Next() {
		var item OutboxItem
		if err := rows.Scan(&item.ID, &item.Kind, &item.Payload, &item.Attempts); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *postgresOutbox) Ack(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM agent.outbox WHERE id=$1`, id)
	return err
}

func (s *postgresOutbox) Fail(ctx context.Context, id int64, nextAt time.Time, dead bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE agent.outbox SET attempts=attempts+1,next_at=$2,dead=$3 WHERE id=$1`, id, nextAt, dead)
	return err
}

func (s *postgresOutbox) Pending(ctx context.Context) (pending, dead int, err error) {
	err = s.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE NOT dead), count(*) FILTER (WHERE dead) FROM agent.outbox`).Scan(&pending, &dead)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	return
}

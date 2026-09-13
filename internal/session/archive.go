package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// Archiver sessiya tarixini o'chirishdan oldin qayta tiklanadigan arxivga ko'chiradi.
type Archiver interface {
	Archive(ctx context.Context, sessionID, reason string) error
	Cleanup(ctx context.Context, olderThan time.Duration) (archived int, deleted int, err error)
}

type archivingStore struct {
	Store
	archiver Archiver
}

// WithArchiveOnReset Reset oldidan tarixni saqlab qolish uchun Store'ni o'raydi.
func WithArchiveOnReset(store Store, archiver Archiver) Store {
	if archiver == nil {
		return store
	}
	return &archivingStore{Store: store, archiver: archiver}
}

func (s *archivingStore) Reset(ctx context.Context, id string) error {
	if err := s.archiver.Archive(ctx, id, "reset"); err != nil {
		return err
	}
	return s.Store.Reset(ctx, id)
}

type archivedSession struct {
	messages []*schema.Message
	state    curtains.SessionState
	reason   string
	at       time.Time
}

func (s *memoryStore) Archive(_ context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions[id]) == 0 {
		if _, ok := s.states[id]; !ok {
			return nil
		}
	}
	s.archived = append(s.archived, archivedSession{messages: append([]*schema.Message(nil), s.sessions[id]...), state: cloneState(s.states[id]), reason: reason, at: time.Now()})
	delete(s.sessions, id)
	delete(s.states, id)
	delete(s.owners, id)
	delete(s.updated, id)
	return nil
}

func (s *memoryStore) Cleanup(_ context.Context, olderThan time.Duration) (int, int, error) {
	cutoff := time.Now().Add(-olderThan)
	archived, deleted := 0, 0
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.sessions {
		if s.updated[id].After(cutoff) {
			continue
		}
		s.archived = append(s.archived, archivedSession{messages: append([]*schema.Message(nil), s.sessions[id]...), state: cloneState(s.states[id]), reason: "ttl", at: time.Now()})
		delete(s.sessions, id)
		delete(s.states, id)
		delete(s.updated, id)
		archived++
	}
	kept := s.archived[:0]
	for _, item := range s.archived {
		if item.at.Before(cutoff) {
			deleted++
		} else {
			kept = append(kept, item)
		}
	}
	s.archived = kept
	return archived, deleted, nil
}

func (s *memoryStore) ArchivedCount() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.archived) }

func (s *pgStore) ensureArchiveTable(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS agent.archived_messages (
	 id BIGSERIAL PRIMARY KEY, session_id TEXT NOT NULL, user_id TEXT NOT NULL DEFAULT '', tenant_id TEXT NOT NULL DEFAULT '',
	 messages JSONB NOT NULL, state JSONB NOT NULL, archived_at TIMESTAMPTZ NOT NULL DEFAULT now(), reason TEXT NOT NULL)`)
	if err != nil {
		return fmt.Errorf("arxiv jadvali: %w", err)
	}
	return nil
}

func (s *pgStore) Archive(ctx context.Context, id, reason string) error {
	if err := s.ensureArchiveTable(ctx); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("arxiv tranzaksiyasi: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID, tenantID string
	var messages, state []byte
	err = tx.QueryRow(ctx, `SELECT user_id,tenant_id,messages,state FROM agent.sessions WHERE session_id=$1 FOR UPDATE`, id).Scan(&userID, &tenantID, &messages, &state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("arxiv sessiyasi: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO agent.archived_messages(session_id,user_id,tenant_id,messages,state,reason) VALUES($1,$2,$3,$4::jsonb,$5::jsonb,$6)`, id, userID, tenantID, messages, state, reason); err != nil {
		return fmt.Errorf("arxiv yozish: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM agent.sessions WHERE session_id=$1`, id); err != nil {
		return fmt.Errorf("sessiyani arxivdan o'chirish: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("arxivni yakunlash: %w", err)
	}
	return nil
}

func (s *pgStore) Cleanup(ctx context.Context, olderThan time.Duration) (archived, deleted int, err error) {
	if err = s.ensureArchiveTable(ctx); err != nil {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	rows, err := s.pool.Query(ctx, `SELECT session_id FROM agent.sessions WHERE updated_at < $1`, cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("eski sessiyalar: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return
	}
	for _, id := range ids {
		if err = s.Archive(ctx, id, "ttl"); err != nil {
			return
		}
		archived++
	}
	res, err := s.pool.Exec(ctx, `DELETE FROM agent.archived_messages WHERE archived_at < $1`, cutoff)
	if err != nil {
		return archived, 0, fmt.Errorf("eski arxivlar: %w", err)
	}
	deleted = int(res.RowsAffected())
	return
}

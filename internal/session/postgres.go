package session

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// errSessionOwnerMismatch — so'rov egasi sessiya egasi bilan mos kelmadi. Postgres va
// in-memory store bitta sentinel'ni qaytaradi (rules §10.4: boshqa sotuvchi suhbatni
// o'qiy ham, yoza ham olmaydi) — shu sababli xato matni va kelib chiqishi bir xil.
var errSessionOwnerMismatch = errors.New("sessiya egasi mos emas")

type PostgresOptions struct{ AutoMigrate bool }

type pgStore struct{ pool *pgxpool.Pool }

func (s *pgStore) Owner(ctx context.Context, id string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM agent.sessions WHERE session_id=$1`, id).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sessiya egasini o'qish: %w", err)
	}
	return userID, nil
}

func NewPostgres(ctx context.Context, dsn string, options ...PostgresOptions) (Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if len(options) > 0 && options[0].AutoMigrate {
		if err := applyMigrations(ctx, pool); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return &pgStore{pool: pool}, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS agent;
CREATE TABLE IF NOT EXISTS agent.schema_migrations (version INT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("migratsiya jadvali: %w", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migratsiya tranzaksiyasi: %w", err)
	}
	defer tx.Rollback(ctx)
	const version = 1
	var applied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent.schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
		return fmt.Errorf("migratsiya versiyasi: %w", err)
	}
	if !applied {
		raw, err := migrationFiles.ReadFile("migrations/0001_sessions.sql")
		if err != nil {
			return fmt.Errorf("migratsiyani o'qish: %w", err)
		}
		if _, err := tx.Exec(ctx, string(raw)); err != nil {
			return fmt.Errorf("0001 migratsiya: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO agent.schema_migrations(version) VALUES ($1)`, version); err != nil {
			return fmt.Errorf("migratsiyani belgilash: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migratsiyani yakunlash: %w", err)
	}
	return nil
}

func (s *pgStore) History(ctx context.Context, id string) ([]*schema.Message, error) {
	ok, err := s.ownerMatches(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		slog.Warn("sessiya egasi mos emas", "session_id", id)
		return nil, nil
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT messages FROM agent.sessions WHERE session_id=$1`, id).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("tarixni o'qish: %w", err)
	}
	var out []*schema.Message
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("tarixni tahlil qilish: %w", err)
	}
	return out, nil
}

func (s *pgStore) Append(ctx context.Context, id string, msgs ...*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	raw, err := json.Marshal(msgs)
	if err != nil {
		return fmt.Errorf("xabarlarni JSON'ga o'girish: %w", err)
	}
	userID, tenantID := owner(ctx)
	if _, err := s.pool.Exec(ctx, `INSERT INTO agent.sessions(session_id,user_id,tenant_id) VALUES($1,$2,$3) ON CONFLICT (session_id) DO NOTHING`, id, userID, tenantID); err != nil {
		return fmt.Errorf("sessiyani yaratish: %w", err)
	}
	ok, err := s.ownerMatches(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return errSessionOwnerMismatch
	}
	if _, err := s.pool.Exec(ctx, `UPDATE agent.sessions SET messages=messages || $2::jsonb, updated_at=now() WHERE session_id=$1`, id, raw); err != nil {
		return fmt.Errorf("tarixni saqlash: %w", err)
	}
	return nil
}

func (s *pgStore) State(ctx context.Context, id string) (curtains.SessionState, error) {
	ok, err := s.ownerMatches(ctx, id)
	if err != nil {
		return curtains.NewSessionState(), err
	}
	if !ok {
		slog.Warn("sessiya egasi mos emas", "session_id", id)
		return curtains.NewSessionState(), nil
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT state FROM agent.sessions WHERE session_id=$1`, id).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return curtains.NewSessionState(), nil
		}
		return curtains.NewSessionState(), fmt.Errorf("state o'qish: %w", err)
	}
	if len(raw) == 0 || string(raw) == "{}" {
		return curtains.NewSessionState(), nil
	}
	var st curtains.SessionState
	if err := json.Unmarshal(raw, &st); err != nil {
		return curtains.NewSessionState(), fmt.Errorf("state tahlili: %w", err)
	}
	if st.Stage == "" {
		st.Stage = curtains.StageNeed
	}
	return st, nil
}

func (s *pgStore) SetState(ctx context.Context, id string, st curtains.SessionState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("state JSON'ga o'girish: %w", err)
	}
	userID, tenantID := owner(ctx)
	if _, err := s.pool.Exec(ctx, `INSERT INTO agent.sessions(session_id,user_id,tenant_id) VALUES($1,$2,$3) ON CONFLICT (session_id) DO NOTHING`, id, userID, tenantID); err != nil {
		return fmt.Errorf("sessiyani yaratish: %w", err)
	}
	ok, err := s.ownerMatches(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return errSessionOwnerMismatch
	}
	if _, err := s.pool.Exec(ctx, `UPDATE agent.sessions SET state=$2::jsonb, updated_at=now() WHERE session_id=$1`, id, raw); err != nil {
		return fmt.Errorf("state saqlash: %w", err)
	}
	return nil
}

func (s *pgStore) Reset(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `UPDATE agent.sessions SET messages='[]'::jsonb, state='{}'::jsonb, updated_at=now() WHERE session_id=$1`, id); err != nil {
		return fmt.Errorf("sessiyani tozalash: %w", err)
	}
	return nil
}

// ownerMatches — so'rov egasi sessiya egasi bilan mos keladimi (rules §10.4).
// Yozuv yo'q bo'lsa ruxsat beriladi: sessiya birinchi yozuvda o'z egasini oladi.
// Bo'sh user_id ham shu qoidaga bo'ysunadi — u FAQAT egasiz (user_id bo'sh) sessiyaga
// kira oladi. Aks holda (fail-open) har qanday so'rov boshqa sotuvchi suhbatini
// o'qirdi: sessiya izolyatsiyasi butunlay yo'qolgan bo'lardi.
func (s *pgStore) ownerMatches(ctx context.Context, id string) (bool, error) {
	var storedUser, storedTenant string
	err := s.pool.QueryRow(ctx, `SELECT user_id,tenant_id FROM agent.sessions WHERE session_id=$1`, id).Scan(&storedUser, &storedTenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("sessiya egasini tekshirish: %w", err)
	}
	return ownerAllows(sessionOwner{user: storedUser, tenant: storedTenant}, true, ownerOf(ctx)), nil
}

// sessionOwner — sessiya egasining (user_id, tenant_id) juftligi; Postgres'dagi
// agent.sessions ustunlariga mos. Ikkalasini ham saqlash shart: bitta tenant ichida
// ko'p sotuvchi (user_id) ishlaydi, tenant esa kelganda ajratadi.
type sessionOwner struct {
	user   string
	tenant string
}

// ownerAllows — egalik qarorining YAGONA qoidasi (Postgres va in-memory store shuni
// ishlatadi, aks holda chalkashlik chiqadi va xavfsizlik nuqsoni yashirin bo'lib qoladi):
// yozuv yo'q (yoki egasiz) — ruxsat (yangi sessiya), yozuv bor — faqat aniq moslik.
func ownerAllows(stored sessionOwner, hasOwner bool, want sessionOwner) bool {
	if !hasOwner {
		return true
	}
	return stored == want
}

func owner(ctx context.Context) (string, string) {
	if u, ok := curtains.UserFromContext(ctx); ok {
		return strings.TrimSpace(u.UserID), strings.TrimSpace(u.TenantID)
	}
	return "", ""
}

// ownerOf — so'rov kontekstidagi ega (`owner` dan juftlikka yig'iladi).
func ownerOf(ctx context.Context) sessionOwner {
	userID, tenantID := owner(ctx)
	return sessionOwner{user: userID, tenant: tenantID}
}

func (s *pgStore) Close() { s.pool.Close() }

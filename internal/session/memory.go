package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// memoryStore Store'ning jarayon-ichi (in-memory) implementatsiyasi.
// Egalik semantikasi Postgres bilan BIR XIL (rules §10.4): birinchi yozuvda
// (user_id, tenant_id) egasi qayd etiladi, keyingi o'qish/yozishda mos kelishi
// majburiy. Aks holda dev/e2e rejimida xavfsizlik nuqsoni faqat prod'da tuzatilgan
// bo'lardi — testlar esa yotgan kodni tekshiradi.
type memoryStore struct {
	mu       sync.RWMutex
	sessions map[string][]*schema.Message
	states   map[string]curtains.SessionState
	owners   map[string]sessionOwner
	archived []archivedSession
	updated  map[string]time.Time
}

// NewMemory bo'sh in-memory Store yaratadi.
func NewMemory() Store {
	return &memoryStore{sessions: make(map[string][]*schema.Message), states: make(map[string]curtains.SessionState), owners: make(map[string]sessionOwner), updated: make(map[string]time.Time)}
}

func (s *memoryStore) Owner(_ context.Context, id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owners[id].user, nil
}

// hasSessionLocked — sessiyaning o'zi bormi (tarix yoki state). Postgresda buning
// ekvivalenti `agent.sessions` qatori: `Cleanup` qatorni o'chiradi, `owners` yozuvi
// esa shu store'ga xos xotirada qoladi — yozuv yo'q bo'lsa sessiya yangi hisoblanadi.
func (s *memoryStore) hasSessionLocked(id string) bool {
	if len(s.sessions[id]) > 0 {
		return true
	}
	_, ok := s.states[id]
	return ok
}

// ownerMatchesLocked — o'qish uchun egalik tekshiruvi (pgStore `ownerMatches` bilan
// bir xil qoida `ownerAllows` orqali). Mismatch bo'lsa o'quvchi bo'sh natija oladi —
// xato qaytarilmaydi, chunki bu foydalanuvchi kiritmasi xatosi emas (errors-as-results).
func (s *memoryStore) ownerMatchesLocked(ctx context.Context, id string) bool {
	stored, owned := s.owners[id]
	return ownerAllows(stored, owned && s.hasSessionLocked(id), ownerOf(ctx))
}

// claimOwnerLocked — yozuvdan oldin egalikni tasdiqlaydi va yangi sessiyada egasini
// belgilaydi. Postgresdagi `INSERT ... ON CONFLICT DO NOTHING` + `ownerMatches`
// juftligi: egalik faqat birinchi yozuvda o'rnatiladi, keyin faqat tekshiriladi.
func (s *memoryStore) claimOwnerLocked(ctx context.Context, id string) error {
	want := ownerOf(ctx)
	stored, owned := s.owners[id]
	if !owned || !s.hasSessionLocked(id) {
		s.owners[id] = want
		return nil
	}
	if stored != want {
		return errSessionOwnerMismatch
	}
	return nil
}

func (s *memoryStore) History(ctx context.Context, id string) ([]*schema.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ownerMatchesLocked(ctx, id) {
		slog.Warn("sessiya egasi mos emas", "session_id", id)
		return nil, nil
	}

	msgs := s.sessions[id]
	if len(msgs) == 0 {
		return nil, nil
	}
	// Nusxa qaytaramiz — tashqi kod ichki slice'ni o'zgartira olmasin.
	out := make([]*schema.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *memoryStore) Append(ctx context.Context, id string, msgs ...*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.claimOwnerLocked(ctx, id); err != nil {
		return err
	}
	s.sessions[id] = append(s.sessions[id], msgs...)
	s.updated[id] = time.Now()
	return nil
}

// Reset — tarix va state tozalanadi. Ega O'CHIRILMAYDI: Postgres'dagi Reset qatorni
// o'chirmaydi, faqat messages/state bo'shaydi, shuning uchun `Owner` (transport
// ruxsat darajasi uchun) va navbatdagi egalik saqlanadi.
func (s *memoryStore) Reset(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	delete(s.states, id)
	delete(s.updated, id)
	return nil
}

func (s *memoryStore) State(ctx context.Context, id string) (curtains.SessionState, error) {
	s.mu.RLock()
	allowed := s.ownerMatchesLocked(ctx, id)
	st, ok := s.states[id]
	s.mu.RUnlock()
	if !allowed {
		slog.Warn("sessiya egasi mos emas", "session_id", id)
		return curtains.NewSessionState(), nil
	}
	if !ok {
		return curtains.NewSessionState(), nil
	}
	return cloneState(st), nil
}

func (s *memoryStore) SetState(ctx context.Context, id string, st curtains.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.claimOwnerLocked(ctx, id); err != nil {
		return err
	}
	s.states[id] = cloneState(st)
	s.updated[id] = time.Now()
	return nil
}

func cloneState(st curtains.SessionState) curtains.SessionState {
	raw, err := json.Marshal(st)
	if err != nil {
		return st
	}
	var out curtains.SessionState
	if err := json.Unmarshal(raw, &out); err != nil {
		return st
	}
	return out
}

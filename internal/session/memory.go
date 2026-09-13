package session

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// memoryStore Store'ning jarayon-ichi (in-memory) implementatsiyasi.
type memoryStore struct {
	mu       sync.RWMutex
	sessions map[string][]*schema.Message
	states   map[string]curtains.SessionState
}

// NewMemory bo'sh in-memory Store yaratadi.
func NewMemory() Store {
	return &memoryStore{sessions: make(map[string][]*schema.Message), states: make(map[string]curtains.SessionState)}
}

func (s *memoryStore) History(_ context.Context, id string) ([]*schema.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.sessions[id]
	if len(msgs) == 0 {
		return nil, nil
	}
	// Nusxa qaytaramiz — tashqi kod ichki slice'ni o'zgartira olmasin.
	out := make([]*schema.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *memoryStore) Append(_ context.Context, id string, msgs ...*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = append(s.sessions[id], msgs...)
	return nil
}

func (s *memoryStore) Reset(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	delete(s.states, id)
	return nil
}

func (s *memoryStore) State(_ context.Context, id string) (curtains.SessionState, error) {
	s.mu.RLock()
	st, ok := s.states[id]
	s.mu.RUnlock()
	if !ok {
		return curtains.NewSessionState(), nil
	}
	return cloneState(st), nil
}

func (s *memoryStore) SetState(_ context.Context, id string, st curtains.SessionState) error {
	s.mu.Lock()
	s.states[id] = cloneState(st)
	s.mu.Unlock()
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

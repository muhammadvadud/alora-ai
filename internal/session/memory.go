package session

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// memoryStore Store'ning jarayon-ichi (in-memory) implementatsiyasi.
type memoryStore struct {
	mu       sync.RWMutex
	sessions map[string][]*schema.Message
}

// NewMemory bo'sh in-memory Store yaratadi.
func NewMemory() Store {
	return &memoryStore{sessions: make(map[string][]*schema.Message)}
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
	return nil
}

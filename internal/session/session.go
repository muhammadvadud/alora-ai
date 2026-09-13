// Package session suhbat tarixini (xotira) boshqaradi.
package session

import (
	"context"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// Store sessiya bo'yicha suhbat tarixini saqlaydi.
// Implementatsiyalar: NewMemory (in-memory), NewPostgres (DB).
// ctx va error — DB kabi implementatsiyalar uchun (in-memory ularni e'tiborsiz qoldiradi).
type Store interface {
	// Owner sessiyaning egasini qaytaradi; mavjud bo'lmagan sessiya uchun bo'sh satr.
	Owner(ctx context.Context, id string) (string, error)
	// History sessiyadagi xabarlarni tartib bilan qaytaradi (bo'sh bo'lsa nil).
	History(ctx context.Context, id string) ([]*schema.Message, error)
	// Append sessiyaga xabar(lar) qo'shadi.
	Append(ctx context.Context, id string, msgs ...*schema.Message) error
	// Reset sessiya tarixini tozalaydi.
	Reset(ctx context.Context, id string) error
	State(ctx context.Context, id string) (curtains.SessionState, error)
	SetState(ctx context.Context, id string, st curtains.SessionState) error
}

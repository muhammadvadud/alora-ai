package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrOrderUnavailable — buyurtma xizmati erishib bo'lmadi (Django yotibdi / timeout).
var ErrOrderUnavailable = errors.New("order service unavailable")

// OrderClient — Django Backend.SaveOrderDraft sinxron klienti (rules §9.4).
type OrderClient struct {
	client  alorav1.BackendClient
	timeout time.Duration
}

// NewOrderClient buyurtma qoralamasi klientini yaratadi.
func NewOrderClient(conn *grpc.ClientConn, timeout time.Duration) *OrderClient {
	return &OrderClient{client: alorav1.NewBackendClient(conn), timeout: timeout}
}

// SaveOrderDraft buyurtma qoralamasini Django'ga yozadi. Retry YO'Q (ADR-008) —
// dublikat buyurtma xavfi: sotuvchi resurs yetishmaganda qo'lda boshqaradi.
// Sessiya/user/tenant/conversation ctx'dan olinadi, tool ularni bilmaydi (rules §16.6).
func (c *OrderClient) SaveOrderDraft(ctx context.Context, d curtains.OrderDraft) (curtains.OrderDraftResult, error) {
	var result curtains.OrderDraftResult
	timeout := c.timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := &alorav1.OrderDraftRequest{
		SessionId:     session.IDFromContext(ctx),
		CustomerName:  d.CustomerName,
		CustomerPhone: d.CustomerPhone,
		Address:       d.Address,
		ItemsJson:     d.ItemsJSON,
		Note:          d.Note,
	}
	if u, ok := curtains.UserFromContext(ctx); ok {
		req.UserId, req.TenantId, req.ConversationId = u.UserID, u.TenantID, u.ConversationID
	}

	resp, err := c.client.SaveOrderDraft(callCtx, req)
	if err != nil {
		if status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded ||
			errors.Is(err, context.DeadlineExceeded) {
			return result, fmt.Errorf("%w: %w", ErrOrderUnavailable, err)
		}
		return result, fmt.Errorf("save order draft: %w", err)
	}
	if !resp.GetSuccess() {
		return result, &curtains.ValidationError{Field: "order", Msg: resp.GetError()}
	}
	result.OrderID = resp.GetOrderId()
	result.OrderNumber = resp.GetOrderNumber()
	return result, nil
}

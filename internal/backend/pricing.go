package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrPricingUnavailable = errors.New("pricing service unavailable")

type PricingClient struct {
	client   alorav1.PricingClient
	timeout  time.Duration
	tenantID string
}

// NewPricingClient creates a calculator backed by Django Pricing.CalcPreview.
func NewPricingClient(conn *grpc.ClientConn, timeout time.Duration, tenantID string) *PricingClient {
	return &PricingClient{client: alorav1.NewPricingClient(conn), timeout: timeout, tenantID: tenantID}
}

func (p *PricingClient) Calculate(ctx context.Context, in curtains.CalcInput) (curtains.CalcResult, error) {
	var result curtains.CalcResult
	payload, err := json.Marshal(in)
	if err != nil {
		return result, fmt.Errorf("calc payload: %w", err)
	}
	timeout := p.timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request := &alorav1.CalcPreviewRequest{
		PayloadJson: string(payload), UserRole: string(in.Role), TenantId: p.tenantID,
		SessionId: session.IDFromContext(ctx),
	}
	var response *alorav1.CalcPreviewResponse
	for attempt := 0; attempt < 2; attempt++ {
		response, err = p.client.CalcPreview(callCtx, request)
		if err == nil || status.Code(err) != codes.Unavailable || attempt == 1 {
			break
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-callCtx.Done():
			return result, unavailableError(callCtx.Err())
		}
	}
	if err != nil {
		return result, unavailableError(err)
	}
	if response.GetError() != "" {
		return result, responseError(response)
	}
	if err := json.Unmarshal([]byte(response.GetResultJson()), &result); err != nil {
		return result, fmt.Errorf("calc result: %w", err)
	}
	return result, nil
}

func unavailableError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Unavailable {
		return fmt.Errorf("%w: %w", ErrPricingUnavailable, err)
	}
	return err
}

func responseError(response *alorav1.CalcPreviewResponse) error {
	if response.GetErrorFieldsJson() == "" {
		return &curtains.ValidationError{Field: "calc", Msg: response.GetError()}
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(response.GetErrorFieldsJson()), &fields); err != nil {
		return fmt.Errorf("calc error fields: %w", err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return &curtains.ValidationError{Field: "calc", Msg: response.GetError()}
	}
	return &curtains.ValidationError{Field: keys[0], Msg: fields[keys[0]]}
}

var _ curtains.Calculator = (*PricingClient)(nil)

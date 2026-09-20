package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/observability"
	"github.com/muhammadvadud/alora-ai/internal/session"
	"google.golang.org/protobuf/encoding/protojson"
)

type Clock interface{ Now() time.Time }

// DefaultDeadRetention — dead yozuvlar shuncha vaqtdan keyin tozalanadi. Aks holda
// bir martalik uzilishdan qolgan dead yozuvlar health'ni abadiy "degraded" qilib qo'yadi.
const DefaultDeadRetention = 24 * time.Hour

const purgeInterval = 10 * time.Minute

type Outbox struct {
	store         session.OutboxStore
	client        alorav1.BackendClient
	metrics       *observability.Metrics
	clock         Clock
	logger        *slog.Logger
	deadRetention time.Duration
	lastPurge     time.Time
}

type OutboxOptions struct {
	Metrics *observability.Metrics
	Clock   Clock
	Logger  *slog.Logger
	// DeadRetention — dead yozuvlar qancha saqlanadi (0 → DefaultDeadRetention).
	DeadRetention time.Duration
}

func NewOutbox(store session.OutboxStore, client alorav1.BackendClient, options OutboxOptions) *Outbox {
	if options.Clock == nil {
		options.Clock = curtains.RealClock{}
	}
	if options.DeadRetention <= 0 {
		options.DeadRetention = DefaultDeadRetention
	}
	return &Outbox{store: store, client: client, metrics: options.Metrics, clock: options.Clock, logger: options.Logger, deadRetention: options.DeadRetention}
}

func (o *Outbox) Enqueue(ctx context.Context, kind string, payload []byte) error {
	if o == nil || o.store == nil {
		return fmt.Errorf("outbox sozlanmagan")
	}
	return o.store.Enqueue(ctx, kind, payload)
}

func (o *Outbox) Pending(ctx context.Context) (int, int, error) { return o.store.Pending(ctx) }

func (o *Outbox) Run(ctx context.Context) error {
	if o == nil || o.store == nil || o.client == nil {
		return fmt.Errorf("outbox worker sozlanmagan")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := o.process(ctx); err != nil && ctx.Err() == nil && o.logger != nil {
			o.logger.Error("outbox batch xatosi", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (o *Outbox) process(ctx context.Context) error {
	now := o.now()
	items, err := o.store.Claim(ctx, 50, now)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := o.send(ctx, item); err != nil {
			dead := item.Attempts >= 9
			delay := retryDelay(item.Attempts)
			if err := o.store.Fail(ctx, item.ID, now.Add(delay), dead); err != nil {
				return err
			}
			if dead && o.metrics != nil {
				o.metrics.OutboxDead.Inc()
			}
			if dead && o.logger != nil {
				o.logger.Error("outbox yozuvi dead", "id", item.ID, "kind", item.Kind, "error", err)
			}
			continue
		}
		if err := o.store.Ack(ctx, item.ID); err != nil {
			return err
		}
	}
	o.purgeDead(ctx, now)
	o.updatePending(ctx)
	return nil
}

// purgeDead eskirgan dead yozuvlarni vaqti-vaqti bilan o'chiradi.
func (o *Outbox) purgeDead(ctx context.Context, now time.Time) {
	if !o.lastPurge.IsZero() && now.Sub(o.lastPurge) < purgeInterval {
		return
	}
	o.lastPurge = now
	removed, err := o.store.Purge(ctx, now.Add(-o.deadRetention))
	if err != nil {
		if o.logger != nil {
			o.logger.Warn("outbox dead yozuvlarini tozalab bo'lmadi", "error", err)
		}
		return
	}
	if removed > 0 && o.logger != nil {
		o.logger.Info("outbox eski dead yozuvlari tozalandi", "count", removed, "retention", o.deadRetention.String())
	}
}

func (o *Outbox) send(ctx context.Context, item session.OutboxItem) error {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch item.Kind {
	case "usage":
		var req alorav1.UsageRequest
		if err := protojson.Unmarshal(item.Payload, &req); err != nil {
			return fmt.Errorf("usage payload: %w", err)
		}
		_, err := o.client.ReportUsage(callCtx, &req)
		return err
	case "log":
		var req alorav1.MessageLogRequest
		if err := protojson.Unmarshal(item.Payload, &req); err != nil {
			return fmt.Errorf("log payload: %w", err)
		}
		_, err := o.client.LogMessage(callCtx, &req)
		return err
	default:
		return fmt.Errorf("noma'lum outbox turi %q", item.Kind)
	}
}

func (o *Outbox) updatePending(ctx context.Context) {
	pending, _, err := o.store.Pending(ctx)
	if err == nil && o.metrics != nil {
		o.metrics.OutboxPending.Set(float64(pending))
	}
}

func (o *Outbox) now() time.Time {
	if o.clock != nil {
		return o.clock.Now()
	}
	return curtains.RealClock{}.Now()
}

func retryDelay(attempts int) time.Duration {
	delays := []time.Duration{time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute}
	if attempts <= 0 {
		return delays[0]
	}
	if attempts >= len(delays) {
		return delays[len(delays)-1]
	}
	return delays[attempts-1]
}

func marshalPayload(v any) ([]byte, error) { return json.Marshal(v) }

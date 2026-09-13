package backend

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/observability"
	"google.golang.org/grpc"
)

// SyncOptions sinxronlashni testlarda tezlashtirish va asosiy servisga DI qilish
// uchun alohida saqlanadi; backend proto kontrakti esa bu qatlamda o'zgarmaydi.
type SyncOptions struct {
	PageSize   int32
	TenantID   string
	Metrics    *observability.Metrics
	Logger     *slog.Logger
	BackoffMin time.Duration
	BackoffMax time.Duration
	Jitter     func(time.Duration) time.Duration
}

// CatalogSyncer Django katalogining snapshot va Watch oqimini bitta izchil
// ketma-ketlikda qo'llaydi, shuning uchun View yarim yuklangan holatni ko'rmaydi.
type CatalogSyncer struct {
	client     alorav1.CatalogClient
	catalog    *curtains.Catalog
	pageSize   int32
	tenantID   string
	metrics    *observability.Metrics
	backoffMin time.Duration
	backoffMax time.Duration
	jitter     func(time.Duration) time.Duration
	log        *slog.Logger
}

func NewCatalogSyncer(conn *grpc.ClientConn, catalog *curtains.Catalog, opts SyncOptions) *CatalogSyncer {
	if opts.PageSize <= 0 || opts.PageSize > 1000 {
		opts.PageSize = 500
	}
	if opts.BackoffMin <= 0 {
		opts.BackoffMin = time.Second
	}
	if opts.BackoffMax <= 0 {
		opts.BackoffMax = 30 * time.Second
	}
	if opts.BackoffMax < opts.BackoffMin {
		opts.BackoffMax = opts.BackoffMin
	}
	if opts.Jitter == nil {
		opts.Jitter = jitterDuration
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &CatalogSyncer{
		client: alorav1.NewCatalogClient(conn), catalog: catalog, pageSize: opts.PageSize,
		tenantID: opts.TenantID, metrics: opts.Metrics, backoffMin: opts.BackoffMin,
		backoffMax: opts.BackoffMax, jitter: opts.Jitter, log: opts.Logger,
	}
}

// Snapshot barcha sahifalarni yig'ib, faqat to'liq nusxa tayyor bo'lgach almashtiradi.
func (s *CatalogSyncer) Snapshot(ctx context.Context) error {
	var pages []*alorav1.CatalogSnapshot
	token := ""
	for {
		page, err := s.client.Snapshot(ctx, &alorav1.SnapshotRequest{PageToken: token, PageSize: s.pageSize})
		if err != nil {
			s.setWatch(false)
			return err
		}
		pages = append(pages, page)
		token = page.GetNextPageToken()
		if token == "" {
			break
		}
	}
	snapshot, err := snapshotFromPages(pages)
	if err != nil {
		return err
	}
	s.catalog.ApplySnapshot(snapshot)
	s.setSnapshotMetrics()
	return nil
}

// Run ctx yopilganda kutishni ham bekor qiladi; shu sabab servis shutdown paytida
// Watch goroutine'i va backoff timer'i ortda qolmaydi.
func (s *CatalogSyncer) Run(ctx context.Context) error {
	delay := s.backoffMin
	for {
		if err := ctx.Err(); err != nil {
			s.setWatch(false)
			return nil
		}
		if err := s.Snapshot(ctx); err != nil {
			s.log.Warn("catalog_snapshot_failed", "error", err)
			if !s.wait(ctx, delay) {
				s.setWatch(false)
				return nil
			}
			delay = nextBackoff(delay, s.backoffMax)
			continue
		}
		delay = s.backoffMin
		resync, err := s.watch(ctx)
		if err != nil {
			s.setWatch(false)
			s.log.Warn("catalog_watch_disconnected", "error", err)
			if !s.wait(ctx, delay) {
				s.setWatch(false)
				return nil
			}
			delay = nextBackoff(delay, s.backoffMax)
			continue
		}
		if resync {
			delay = s.backoffMin
			continue
		}
		if ctx.Err() != nil {
			s.setWatch(false)
			return nil
		}
	}
}

func (s *CatalogSyncer) watch(ctx context.Context) (bool, error) {
	stream, err := s.client.Watch(ctx, &alorav1.WatchRequest{FromVersion: s.catalog.Version(), TenantId: s.tenantID})
	if err != nil {
		return false, err
	}
	s.setWatch(true)
	for {
		deltaPB, err := stream.Recv()
		if err != nil {
			return false, err
		}
		if deltaPB.GetEntity() == "catalog" && deltaPB.GetAction() == "FULL_RESYNC" {
			s.setWatch(false)
			s.log.Info("catalog_resync", "reason", "FULL_RESYNC", "version", deltaPB.GetVersion())
			if err := s.Snapshot(ctx); err != nil {
				return false, err
			}
			return true, nil
		}
		delta, err := deltaFromPB(deltaPB)
		if err != nil {
			if errors.Is(err, curtains.ErrUnknownEntity) {
				s.log.Warn("catalog_delta_skipped", "entity", deltaPB.GetEntity(), "error", err)
				continue
			}
			s.log.Warn("catalog_delta_skipped", "entity", deltaPB.GetEntity(), "error", err)
			continue
		}
		if err := s.catalog.ApplyDelta(delta); err != nil {
			if errors.Is(err, curtains.ErrVersionGap) {
				s.setWatch(false)
				s.log.Info("catalog_resync", "reason", "version_gap", "version", delta.Version)
				if snapshotErr := s.Snapshot(ctx); snapshotErr != nil {
					return false, snapshotErr
				}
				return true, nil
			}
			s.log.Warn("catalog_delta_skipped", "entity", delta.Entity, "error", err)
			continue
		}
		s.setSnapshotMetrics()
	}
}

func (s *CatalogSyncer) Ready() (bool, string) {
	if s.catalog.View().Ready() {
		return true, ""
	}
	return false, "katalog hali yuklanmadi"
}

func (s *CatalogSyncer) wait(ctx context.Context, delay time.Duration) bool {
	delay = s.jitter(delay)
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current >= maximum/2 {
		return maximum
	}
	return current * 2
}

func jitterDuration(d time.Duration) time.Duration {
	if d <= 1 {
		return d
	}
	// Kichik tasodifiy og'ish bir nechta agent bir vaqtda qayta ulanib,
	// Django'ga sinxron zarba berishining oldini oladi.
	span := d / 5
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(span*2+1)))
	if err != nil {
		return d
	}
	return d - span + time.Duration(n.Int64())
}

func (s *CatalogSyncer) setWatch(connected bool) {
	if s.metrics == nil {
		return
	}
	if connected {
		s.metrics.CatalogWatchConnected.Set(1)
	} else {
		s.metrics.CatalogWatchConnected.Set(0)
	}
}

func (s *CatalogSyncer) setSnapshotMetrics() {
	if s.metrics == nil {
		return
	}
	v := s.catalog.View()
	s.metrics.CatalogVersion.Set(float64(v.Version))
	if v.Stale {
		s.metrics.CatalogStale.Set(1)
	} else {
		s.metrics.CatalogStale.Set(0)
	}
	if !v.SyncedAt.IsZero() {
		s.metrics.CatalogAgeSeconds.Set(time.Since(v.SyncedAt).Seconds())
	}
}

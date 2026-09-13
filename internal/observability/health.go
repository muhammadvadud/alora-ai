package observability

import (
	"sync/atomic"
)

// degradedOutboxThreshold — outbox navbati shundan oshsa Health "degraded" bo'ladi
// (rules.md §8.7: outbox > 1000).
const degradedOutboxThreshold int64 = 1000

// Readiness — transport (HTTP /readyz, gRPC Health) "tayyor"mi va sababini so'raydi.
// Implementatsiyalar: HealthState (real) yoki test'dagi fake. Yuz-murakkab emas:
// bitta metod — interfeys iste'molchiga HAM oddiy.
type Readiness interface {
	Ready() (bool, string)
}

// HealthState servis umumiy salomatligini atomik saqlaydi. Maydonlar turli
// goroutine'lardan yoziladi (katalog sync, outbox worker, transport), shuning
// uchun atomic — data race'ni istisno qilish (rules §3.5). Qiymatlar hali 0.19'da
// o'rnatilmaydi; katalog (1.33) va outbox (2.6) task'larida ulanadi.
type HealthState struct {
	catalogSynced  atomic.Bool // snapshot to'liq yuklandi (rules §4.1.3)
	catalogStale   atomic.Bool // katalog eskirgan (rules §4.1.5)
	watchConnected atomic.Bool // Catalog.Watch oqimi ulangan (rules §4.1.4)
	outboxPending  atomic.Int64
	outboxDead     atomic.Int64
}

// SetCatalogSynced snapshot yuklash tugaganligini belgilaydi. Tugamaguncha /readyz
// 503 qaytarishi kerak (rules §4.1.3) — transport shu qiymatga qaraydi.
func (h *HealthState) SetCatalogSynced(v bool) { h.catalogSynced.Store(v) }

func (h *HealthState) SetCatalogStale(v bool) { h.catalogStale.Store(v) }

func (h *HealthState) SetWatchConnected(v bool) { h.watchConnected.Store(v) }

func (h *HealthState) SetOutboxPending(n int64) { h.outboxPending.Store(n) }
func (h *HealthState) SetOutboxDead(n int64)    { h.outboxDead.Store(n) }

// Ready Readiness interfeysini qanoatlantiradi: katalog hali yuklanmaguncha not_ready.
// Stale esa "xato emas, ogohlantirish" (rules §4.1.5) — servis xizmatni davom ettiradi.
func (h *HealthState) Ready() (bool, string) {
	if !h.catalogSynced.Load() {
		return false, "katalog hali yuklanmadi"
	}
	return true, ""
}

// Status umumiy holatni beradi: "ok" yoki "degraded". Degraded — katalog eskirgan,
// Watch uzilgan, outbox to'lib qolgan yoki snapshot tugallanmagan (rules §8.7).
// Tafsilotlar map'da — Grafana/alert sharti monitoring uchun bir joyda.
func (h *HealthState) Status() (string, map[string]any) {
	state := map[string]any{
		"catalog_ready":   h.catalogSynced.Load(),
		"catalog_stale":   h.catalogStale.Load(),
		"watch_connected": h.watchConnected.Load(),
		"outbox_pending":  h.outboxPending.Load(),
		"outbox_dead":     h.outboxDead.Load(),
	}
	if !h.catalogSynced.Load() || h.catalogStale.Load() || !h.watchConnected.Load() || h.outboxPending.Load() > degradedOutboxThreshold || h.outboxDead.Load() > 0 {
		return "degraded", state
	}
	return "ok", state
}

package observability

import "testing"

func TestHealthState_Ready_DependsOnCatalogSync(t *testing.T) {
	h := &HealthState{}

	// Katalog snapshoti tugamaguncha tayyor emas (rules §4.1.3).
	if ok, _ := h.Ready(); ok {
		t.Fatal("katalog yuklanmasdan Ready() true qaytardi")
	}

	h.SetCatalogSynced(true)
	if ok, reason := h.Ready(); !ok {
		t.Fatalf("katalog yuklangach Ready() false: %q", reason)
	}
}

func TestHealthState_Status_DegradedConditions(t *testing.T) {
	// healthy — barchasi joyida bo'lgan boshlang'ich holat quruvchisi. Testlarda
	// bitta shartni buzib, Status() ni tekshiramiz (rules §8.7 degradatsiya shartlari).
	healthy := func(h *HealthState) {
		h.SetCatalogSynced(true)
		h.SetCatalogStale(false)
		h.SetWatchConnected(true)
		h.SetOutboxPending(0)
	}

	cases := []struct {
		name   string
		setup  func(h *HealthState)
		status string
	}{
		{"sog'lom", healthy, "ok"},
		{"katalog yuklanmagan", func(h *HealthState) { h.SetWatchConnected(true); h.SetOutboxPending(0) }, "degraded"},
		{"katalog eskirgan", func(h *HealthState) { healthy(h); h.SetCatalogStale(true) }, "degraded"},
		{"watch uzilgan", func(h *HealthState) { healthy(h); h.SetWatchConnected(false) }, "degraded"},
		{"outbox 1001", func(h *HealthState) { healthy(h); h.SetOutboxPending(1001) }, "degraded"},
		// Chegara aniq: > 1000 emas, 1000 bo'lsa hali "ok" (rules §8.7).
		{"outbox 1000 hali ok", func(h *HealthState) { healthy(h); h.SetOutboxPending(1000) }, "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &HealthState{}
			tc.setup(h)
			status, _ := h.Status()
			if status != tc.status {
				t.Fatalf("Status() = %q, %q bo'lishi kerak", status, tc.status)
			}
		})
	}
}

func TestHealthState_Status_DetailMap(t *testing.T) {
	h := &HealthState{}
	h.SetCatalogSynced(true)
	h.SetOutboxPending(42)

	_, detail := h.Status()
	for _, key := range []string{"catalog_ready", "catalog_stale", "watch_connected", "outbox_pending"} {
		if _, ok := detail[key]; !ok {
			t.Fatalf("Status() tafsilotida %q yo'q: %v", key, detail)
		}
	}
}

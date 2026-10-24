package backend

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeCatalogServer struct {
	alorav1.UnimplementedCatalogServer
	mu       sync.Mutex
	pages    map[string]*alorav1.CatalogSnapshot
	requests []*alorav1.WatchRequest
	deltas   []*alorav1.CatalogDelta
	// snapshotCalls — Snapshot RPC chaqirilish soni (resync tekshiruvi uchun).
	snapshotCalls int
	// newActive — true bo'lsa Snapshot yangi (import qilingan) katalogni qaytaradi.
	newActive bool
	// deltaCh — Watch oqimi shu kanal orqali boshqariladi: test delta yuborish
	// vaqtini ANIQ belgilaydi (avvalgi versiya stream ochilishi bilan darvo yuborardi).
	deltaCh chan *alorav1.CatalogDelta
}

func (f *fakeCatalogServer) Snapshot(_ context.Context, req *alorav1.SnapshotRequest) (*alorav1.CatalogSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshotCalls++
	if f.newActive {
		return f.pages["new"], nil
	}
	return f.pages[req.GetPageToken()], nil
}

func (f *fakeCatalogServer) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshotCalls
}

func (f *fakeCatalogServer) activateNew() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newActive = true
}

func (f *fakeCatalogServer) watchStarted() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests) > 0
}

func (f *fakeCatalogServer) Watch(req *alorav1.WatchRequest, stream grpc.ServerStreamingServer[alorav1.CatalogDelta]) error {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	legacy := append([]*alorav1.CatalogDelta(nil), f.deltas...)
	ch := f.deltaCh
	f.mu.Unlock()
	for _, delta := range legacy {
		if err := stream.Send(delta); err != nil {
			return err
		}
	}
	if ch == nil {
		return nil
	}
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case delta, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(delta); err != nil {
				return err
			}
		}
	}
}

func newFakeCatalogClient(t *testing.T, server *fakeCatalogServer) (*grpc.ClientConn, *CatalogSyncer, *curtains.Catalog) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	alorav1.RegisterCatalogServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() { grpcServer.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	catalog := curtains.NewCatalog(nil, 10)
	return conn, NewCatalogSyncer(conn, catalog, SyncOptions{PageSize: 100}), catalog
}

func TestCatalogSyncer_SnapshotPages(t *testing.T) {
	server := &fakeCatalogServer{pages: map[string]*alorav1.CatalogSnapshot{
		"":  {Version: 7, Products: []*alorav1.CatalogProduct{{Id: "p1"}}, NextPageToken: "2"},
		"2": {Version: 7, Products: []*alorav1.CatalogProduct{{Id: "p2"}}, NextPageToken: "4"},
		"4": {Version: 7, Products: []*alorav1.CatalogProduct{{Id: "p3"}}},
	}}
	conn, syncer, catalog := newFakeCatalogClient(t, server)
	_ = conn
	if err := syncer.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if catalog.Version() != 7 || len(catalog.View().Products()) != 3 {
		t.Fatalf("version=%d products=%d", catalog.Version(), len(catalog.View().Products()))
	}
}

func TestCatalogSyncer_DeltaAndReady(t *testing.T) {
	server := &fakeCatalogServer{pages: map[string]*alorav1.CatalogSnapshot{"": {Version: 1, Products: []*alorav1.CatalogProduct{{Id: "p1"}}}}, deltas: []*alorav1.CatalogDelta{{Version: 2, Entity: "product", EntityId: "p2", Action: "create", PayloadJson: `{"id":"p2","name":"Yangi"}`}}}
	_, syncer, catalog := newFakeCatalogClient(t, server)
	if err := syncer.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = syncer.Run(ctx); close(done) }()
	for i := 0; i < 100 && catalog.Version() < 2; i++ {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if _, ok := catalog.View().Product("p2"); !ok {
		t.Fatal("delta product qo'llanmadi")
	}
	if ready, _ := syncer.Ready(); !ready {
		t.Fatal("catalog tayyor emas")
	}
}

// TestCatalogSyncer_CatalogEntityAlwaysResyncs — B3 (ADR-020).
//
// 2026-09-28 live: Django katta import'dan keyin `entity="catalog"` +
// `action="bulk_import"` yuboradi. Bu "FULL_RESYNC" emas, shuning uchun eski kod
// deltani qo'llashga urinib View'ni yarim qoldirgan — import bajarilgan katalog
// BO'SH ko'rinardi va keyin "eskirgan" degan ogohlantirish keldi.
//
// entity="catalog" — katalog butunligi o'zgargan, demi har ikkala action ham
// to'liq Snapshot qilishi kerak.
func TestCatalogSyncer_CatalogEntityAlwaysResyncs(t *testing.T) {
	for _, action := range []string{"FULL_RESYNC", "bulk_import"} {
		t.Run(action, func(t *testing.T) {
			server := &fakeCatalogServer{
				pages: map[string]*alorav1.CatalogSnapshot{
					// boshlang'ich (eski) katalog: bitta mahsulot
					"": {Version: 1, Products: []*alorav1.CatalogProduct{{Id: "p-old"}}},
					// importdan keyingi katalog: boshqa to'plam
					"new": {Version: 9, Products: []*alorav1.CatalogProduct{{Id: "p-new-1"}, {Id: "p-new-2"}}},
				},
				deltaCh: make(chan *alorav1.CatalogDelta, 1),
			}
			_, syncer, catalog := newFakeCatalogClient(t, server)
			if err := syncer.Snapshot(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, ok := catalog.View().Product("p-old"); !ok {
				t.Fatal("boshlang'ich snapshot qo'llanmadi")
			}

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { _ = syncer.Run(ctx); close(done) }()

			// Watch ochilishini kutamiz — undan keyin delta yuboramiz
			waitFor(t, server.watchStarted, "watch ochilmadi")

			// Server endi import qilingan katalogni qaytaradi
			server.activateNew()
			server.deltaCh <- &alorav1.CatalogDelta{Version: 9, Entity: "catalog", Action: action}

			waitFor(t, func() bool { return catalog.Version() == 9 }, "katalog yangilanmadi")
			cancel()
			<-done

			// To'liq resync: eski mahsulot KETMASI kerak
			if _, ok := catalog.View().Product("p-old"); ok {
				t.Fatal("eski mahsulot saqlanib qoldi — to'liq resync bajarilmagan")
			}
			for _, id := range []string{"p-new-1", "p-new-2"} {
				if _, ok := catalog.View().Product(id); !ok {
					t.Fatalf("import qilingan %s yo'q", id)
				}
			}
		})
	}
}

// B3: entity="catalog" bo'lmagan delta (oddiy product create) resync qilmaydi —
// faqat o'zini qo'llaydi. Aks holda har mahsulot o'zgarishida butun katalog
// qayta yuklanar edi.
func TestCatalogSyncer_ProductDeltaDoesNotResync(t *testing.T) {
	server := &fakeCatalogServer{
		pages:   map[string]*alorav1.CatalogSnapshot{"": {Version: 1, Products: []*alorav1.CatalogProduct{{Id: "p1"}}}},
		deltaCh: make(chan *alorav1.CatalogDelta, 1),
	}
	_, syncer, catalog := newFakeCatalogClient(t, server)
	if err := syncer.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = syncer.Run(ctx); close(done) }()

	waitFor(t, server.watchStarted, "watch ochilmadi")
	baseline := server.calls() // Run ning o'z loop Snapshot'i + qo'lda chaqirilgan

	server.deltaCh <- &alorav1.CatalogDelta{Version: 2, Entity: "product", EntityId: "p2", Action: "create", PayloadJson: `{"id":"p2","name":"Yangi"}`}
	waitFor(t, func() bool { return catalog.Version() == 2 }, "product delta qo'llanmadi")
	cancel()
	<-done

	if _, ok := catalog.View().Product("p2"); !ok {
		t.Fatal("product delta qo'llanmadi")
	}
	if got := server.calls(); got != baseline {
		t.Fatalf("oddiy delta Snapshot qayta chaqirmasligi kerak: %d -> %d", baseline, got)
	}
}

// waitFor — shart vaqt chegarasida ro'yl bo'lishini kutadi.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(msg)
}

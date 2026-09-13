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
}

func (f *fakeCatalogServer) Snapshot(_ context.Context, req *alorav1.SnapshotRequest) (*alorav1.CatalogSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pages[req.GetPageToken()], nil
}

func (f *fakeCatalogServer) Watch(req *alorav1.WatchRequest, stream grpc.ServerStreamingServer[alorav1.CatalogDelta]) error {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	deltas := append([]*alorav1.CatalogDelta(nil), f.deltas...)
	f.mu.Unlock()
	for _, delta := range deltas {
		if err := stream.Send(delta); err != nil {
			return err
		}
	}
	return nil
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

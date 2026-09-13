package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/config"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func ServerCredentials(cfg config.TLSConfig) (credentials.TransportCredentials, error) {
	if !cfg.Enabled {
		return insecure.NewCredentials(), nil
	}
	cert, err := tls.LoadX509KeyPair(envFile(cfg.CertFileEnv), envFile(cfg.KeyFileEnv))
	if err != nil {
		return nil, fmt.Errorf("grpc server sertifikati: %w", err)
	}
	ca, err := readCAPool(envFile(cfg.CAFileEnv))
	if err != nil {
		return nil, fmt.Errorf("grpc server CA: %w", err)
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, ClientCAs: ca, ClientAuth: tls.RequireAndVerifyClientCert}), nil
}

func envFile(name string) string { return os.Getenv(name) }

func readCAPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("CA PEM topilmadi")
	}
	return pool, nil
}

// ServeTLS server konfiguratsiyasining qolgan qismini mavjud Serve bilan bir xil saqlaydi.
func ServeTLS(ctx context.Context, addr string, srv *Server, cfg ServeConfig, creds credentials.TransportCredentials) error {
	maxSessions := cfg.MaxSessions
	if maxSessions <= 0 {
		maxSessions = 100
	}
	limiter := newRateLimiter(cfg.RateLimitPerMinute)
	serverOptions := []grpcgo.ServerOption{
		grpcgo.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second}),
		grpcgo.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}),
		grpcgo.MaxRecvMsgSize(4 << 20), grpcgo.MaxSendMsgSize(4 << 20), grpcgo.MaxConcurrentStreams(uint32(maxSessions)),
		grpcgo.ChainUnaryInterceptor(unaryInterceptors(interceptorOptions{token: cfg.ServiceToken, authRequired: cfg.AuthRequired, metrics: cfg.Metrics, limiter: limiter})...),
		grpcgo.ChainStreamInterceptor(streamInterceptors(interceptorOptions{token: cfg.ServiceToken, authRequired: cfg.AuthRequired, metrics: cfg.Metrics, limiter: limiter})...),
	}
	if creds != nil {
		serverOptions = append(serverOptions, grpcgo.Creds(creds))
	}
	server := grpcgo.NewServer(serverOptions...)
	alorav1.RegisterAgentServer(server, srv)
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case err := <-serveErr:
		if errors.Is(err, grpcgo.ErrServerStopped) {
			return nil
		}
		return err
	case <-ctx.Done():
		server.GracefulStop()
		return nil
	}
}

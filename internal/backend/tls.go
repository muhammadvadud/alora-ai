package backend

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/muhammadvadud/alora-ai/internal/config"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func ClientCredentials(cfg config.TLSConfig) (credentials.TransportCredentials, error) {
	if !cfg.Enabled {
		return insecure.NewCredentials(), nil
	}
	caBytes, err := os.ReadFile(os.Getenv(cfg.CAFileEnv))
	if err != nil {
		return nil, fmt.Errorf("backend CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("backend CA PEM topilmadi")
	}
	cert, err := tls.LoadX509KeyPair(os.Getenv(cfg.CertFileEnv), os.Getenv(cfg.KeyFileEnv))
	if err != nil {
		return nil, fmt.Errorf("backend klient sertifikati: %w", err)
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: []tls.Certificate{cert}, ServerName: os.Getenv("ALORA_BACKEND_TLS_SERVER_NAME")}), nil
}

package grpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/config"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestServerCredentialsMTLS(t *testing.T) {
	ca, caKey := newCert(t, nil, nil, "alora-ca", true)
	dir := t.TempDir()
	// Generate the key/certificate pairs together so the test also verifies LoadX509KeyPair.
	serverCertPath, serverKeyPath := makePair(t, dir, "server", ca, caKey, "localhost", false)
	clientCertPath, clientKeyPath := makePair(t, dir, "client", ca, caKey, "alora-client", false)
	t.Setenv("TEST_GRPC_CERT", serverCertPath)
	t.Setenv("TEST_GRPC_KEY", serverKeyPath)
	t.Setenv("TEST_GRPC_CA", filepath.Join(dir, "ca-real.crt"))
	os.WriteFile(filepath.Join(dir, "ca-real.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca}), 0600)
	creds, err := ServerCredentials(config.TLSConfig{Enabled: true, CertFileEnv: "TEST_GRPC_CERT", KeyFileEnv: "TEST_GRPC_KEY", CAFileEnv: "TEST_GRPC_CA"})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := grpcgo.NewServer(grpcgo.Creds(creds))
	alorav1.RegisterAgentServer(s, New(nil, Options{}))
	go s.Serve(ln)
	defer s.Stop()
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(mustRead(t, filepath.Join(dir, "ca-real.crt")))
	clientTLS := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: []tls.Certificate{loadPair(t, clientCertPath, clientKeyPath)}, ServerName: "localhost"})
	conn, err := grpcgo.NewClient(ln.Addr().String(), grpcgo.WithTransportCredentials(clientTLS))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := alorav1.NewAgentClient(conn).Health(context.Background(), &alorav1.Empty{}); err != nil {
		t.Fatalf("mTLS Health: %v", err)
	}
	bad, err := grpcgo.NewClient(ln.Addr().String(), grpcgo.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, ServerName: "localhost"})))
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Close()
	if _, err := alorav1.NewAgentClient(bad).Health(context.Background(), &alorav1.Empty{}); err == nil {
		t.Fatal("sertifikatsiz klient qabul qilindi")
	}
}

func TestServerCredentialsDisabled(t *testing.T) {
	if _, err := ServerCredentials(config.TLSConfig{}); err != nil {
		t.Fatal(err)
	}
}

func newCert(t *testing.T, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, name string, ca bool) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tmpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, IsCA: ca, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment}
	if ca {
		tmpl.KeyUsage |= x509.KeyUsageCertSign
	}
	if name == "localhost" {
		tmpl.DNSNames = []string{"localhost"}
	}
	if parent == nil {
		parent, parentKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	return der, key
}

func makePair(t *testing.T, dir, name string, ca []byte, caKey *ecdsa.PrivateKey, cn string, isCA bool) (string, string) {
	cert, key := newCert(t, mustParseCert(t, ca), caKey, cn, isCA)
	return writePEM(t, dir, name+".crt", "CERTIFICATE", cert), writeKey(t, dir, name+".key", key)
}
func mustParseCert(t *testing.T, der []byte) *x509.Certificate {
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func writePEM(t *testing.T, dir, name, typ string, der []byte) string {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func writeKey(t *testing.T, dir, name string, key *ecdsa.PrivateKey) string {
	der, _ := x509.MarshalECPrivateKey(key)
	return writePEM(t, dir, name, "EC PRIVATE KEY", der)
}
func loadPair(t *testing.T, cert, key string) tls.Certificate {
	p, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

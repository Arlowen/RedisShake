package redischeck

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"RedisShake/internal/client/proto"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
)

type fakeRedisServer struct {
	listener       net.Listener
	password       string
	clusterEnabled bool
	denyWrite      bool
	sentinelMaster string
	redirectTarget string
	deletedKeys    atomic.Int64
	closed         chan struct{}
	waitGroup      sync.WaitGroup
}

type fakeRedisSettings struct {
	password       string
	clusterEnabled bool
	denyWrite      bool
	sentinelMaster string
	redirectTarget string
	tlsConfig      *tls.Config
}

func TestCheckerSourceAndTarget(t *testing.T) {
	server := newFakeRedisServer(t, fakeRedisSettings{})
	checker := Checker{Timeout: time.Second}
	resolved := connections.Resolved{Spec: connections.Spec{
		Name:     "Redis",
		Topology: domain.TopologyStandalone,
		Address:  server.listener.Addr().String(),
	}}

	source := checker.Check(context.Background(), resolved, connections.TestPurposeSource)
	if !source.Success || source.ServerProduct != "Redis" || source.ServerVersion != "7.2.0" || source.Role != "master" {
		t.Fatalf("source result = %+v", source)
	}
	if source.ClusterEnabled {
		t.Fatal("source ClusterEnabled = true")
	}

	target := checker.Check(context.Background(), resolved, connections.TestPurposeTarget)
	if !target.Success || !hasCheck(target, "target_write", connections.CheckStatePass) {
		t.Fatalf("target result = %+v", target)
	}
	if server.deletedKeys.Load() != 1 {
		t.Fatalf("deleted test keys = %d, want 1", server.deletedKeys.Load())
	}
}

func TestCheckerReportsAuthenticationAndTopologyFailures(t *testing.T) {
	authServer := newFakeRedisServer(t, fakeRedisSettings{password: "correct-password"})
	checker := Checker{Timeout: time.Second}
	authResult := checker.Check(context.Background(), connections.Resolved{Spec: connections.Spec{
		Name:     "Protected",
		Topology: domain.TopologyStandalone,
		Address:  authServer.listener.Addr().String(),
		Password: "wrong-password",
	}}, connections.TestPurposeSource)
	if authResult.Success || !hasCheck(authResult, "authentication", connections.CheckStateFail) {
		t.Fatalf("auth result = %+v", authResult)
	}

	clusterServer := newFakeRedisServer(t, fakeRedisSettings{clusterEnabled: true})
	topologyResult := checker.Check(context.Background(), connections.Resolved{Spec: connections.Spec{
		Name:     "Wrong topology",
		Topology: domain.TopologyStandalone,
		Address:  clusterServer.listener.Addr().String(),
	}}, connections.TestPurposeSource)
	if topologyResult.Success || !hasCheck(topologyResult, "topology", connections.CheckStateFail) {
		t.Fatalf("topology result = %+v", topologyResult)
	}
}

func TestCheckerResolvesSentinel(t *testing.T) {
	master := newFakeRedisServer(t, fakeRedisSettings{})
	sentinel := newFakeRedisServer(t, fakeRedisSettings{sentinelMaster: master.listener.Addr().String()})
	checker := Checker{Timeout: time.Second}
	result := checker.Check(context.Background(), connections.Resolved{Spec: connections.Spec{
		Name:     "Sentinel Redis",
		Topology: domain.TopologySentinel,
		Sentinel: connections.SentinelConfig{
			Address:    sentinel.listener.Addr().String(),
			MasterName: "mymaster",
		},
	}}, connections.TestPurposeSource)
	if !result.Success || result.EffectiveAddress != master.listener.Addr().String() {
		t.Fatalf("sentinel result = %+v", result)
	}
	if !hasCheck(result, "sentinel_resolution", connections.CheckStatePass) {
		t.Fatalf("sentinel result missing resolution check: %+v", result)
	}
}

func TestCheckerTargetWriteFailure(t *testing.T) {
	server := newFakeRedisServer(t, fakeRedisSettings{denyWrite: true})
	result := (&Checker{Timeout: time.Second}).Check(context.Background(), connections.Resolved{Spec: connections.Spec{
		Name:     "Read only",
		Topology: domain.TopologyStandalone,
		Address:  server.listener.Addr().String(),
	}}, connections.TestPurposeTarget)
	if result.Success || !hasCheck(result, "target_write", connections.CheckStateFail) {
		t.Fatalf("target write result = %+v", result)
	}
}

func TestCheckerFollowsClusterMovedForTargetWrite(t *testing.T) {
	target := newFakeRedisServer(t, fakeRedisSettings{clusterEnabled: true})
	seed := newFakeRedisServer(t, fakeRedisSettings{clusterEnabled: true, redirectTarget: target.listener.Addr().String()})
	result := (&Checker{Timeout: time.Second}).Check(context.Background(), connections.Resolved{Spec: connections.Spec{
		Name:     "Cluster",
		Topology: domain.TopologyCluster,
		Address:  seed.listener.Addr().String(),
	}}, connections.TestPurposeTarget)
	if !result.Success || !hasCheck(result, "cluster_redirect", connections.CheckStatePass) {
		t.Fatalf("cluster target result = %+v", result)
	}
	if target.deletedKeys.Load() != 1 {
		t.Fatalf("cluster target deleted keys = %d, want 1", target.deletedKeys.Load())
	}
}

func TestBuildTLSConfigRejectsInvalidCA(t *testing.T) {
	_, err := buildTLSConfig("redis.example.com:6379", connections.TLSConfig{
		Enabled:   true,
		CACertPEM: "not-a-certificate",
	})
	if err == nil {
		t.Fatal("buildTLSConfig() accepted an invalid CA certificate")
	}
}

func TestCheckerTLSCertificateVerification(t *testing.T) {
	serverTLS, caPEM := testServerCertificate(t, "redis.test")
	server := newFakeRedisServer(t, fakeRedisSettings{tlsConfig: serverTLS})
	checker := Checker{Timeout: time.Second}
	resolved := connections.Resolved{Spec: connections.Spec{
		Name:     "TLS Redis",
		Topology: domain.TopologyStandalone,
		Address:  server.listener.Addr().String(),
		TLS: connections.TLSConfig{
			Enabled:    true,
			ServerName: "redis.test",
			CACertPEM:  caPEM,
		},
	}}
	result := checker.Check(context.Background(), resolved, connections.TestPurposeSource)
	if !result.Success {
		t.Fatalf("TLS result = %+v", result)
	}

	resolved.TLS.ServerName = "wrong.test"
	wrongName := checker.Check(context.Background(), resolved, connections.TestPurposeSource)
	if wrongName.Success || !hasCheck(wrongName, "tls_handshake", connections.CheckStateFail) {
		t.Fatalf("wrong TLS server name result = %+v", wrongName)
	}
}

func newFakeRedisServer(t *testing.T, settings fakeRedisSettings) *fakeRedisServer {
	t.Helper()
	var listener net.Listener
	var err error
	if settings.tlsConfig != nil {
		listener, err = tls.Listen("tcp", "127.0.0.1:0", settings.tlsConfig)
	} else {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &fakeRedisServer{
		listener:       listener,
		password:       settings.password,
		clusterEnabled: settings.clusterEnabled,
		denyWrite:      settings.denyWrite,
		sentinelMaster: settings.sentinelMaster,
		redirectTarget: settings.redirectTarget,
		closed:         make(chan struct{}),
	}
	server.waitGroup.Add(1)
	go server.serve()
	t.Cleanup(func() {
		close(server.closed)
		_ = server.listener.Close()
		server.waitGroup.Wait()
	})
	return server
}

func testServerCertificate(t *testing.T, dnsName string) (*tls.Config, string) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: dnsName},
		DNSNames:              []string{dnsName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatalf("tls.X509KeyPair() error = %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, string(certificatePEM)
}

func (s *fakeRedisServer) serve() {
	defer s.waitGroup.Done()
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
				continue
			}
		}
		s.waitGroup.Add(1)
		go func() {
			defer s.waitGroup.Done()
			s.serveConnection(connection)
		}()
	}
}

func (s *fakeRedisServer) serveConnection(connection net.Conn) {
	defer connection.Close()
	reader := proto.NewReader(bufio.NewReader(connection))
	writer := bufio.NewWriter(connection)
	authenticated := s.password == ""
	for {
		reply, err := reader.ReadReply()
		if err != nil {
			return
		}
		items, ok := reply.([]interface{})
		if !ok || len(items) == 0 {
			writeErrorReply(writer, "ERR invalid command")
			continue
		}
		command, _ := items[0].(string)
		command = strings.ToUpper(command)
		if command == "AUTH" {
			candidate, _ := items[len(items)-1].(string)
			if candidate == s.password {
				authenticated = true
				writeStatusReply(writer, "OK")
			} else {
				writeErrorReply(writer, "WRONGPASS invalid username-password pair")
			}
			continue
		}
		if !authenticated {
			writeErrorReply(writer, "NOAUTH Authentication required")
			continue
		}
		switch command {
		case "PING":
			writeStatusReply(writer, "PONG")
		case "INFO":
			section := ""
			if len(items) > 1 {
				section, _ = items[1].(string)
			}
			switch strings.ToLower(section) {
			case "server":
				writeBulkReply(writer, "redis_version:7.2.0\r\n")
			case "replication":
				writeBulkReply(writer, "role:master\r\n")
			case "cluster":
				enabled := "0"
				if s.clusterEnabled {
					enabled = "1"
				}
				writeBulkReply(writer, "cluster_enabled:"+enabled+"\r\n")
			default:
				writeBulkReply(writer, "")
			}
		case "SET":
			if s.denyWrite {
				writeErrorReply(writer, "NOPERM this user has no permissions to run SET")
			} else if s.redirectTarget != "" {
				writeErrorReply(writer, "MOVED 1234 "+s.redirectTarget)
			} else {
				writeStatusReply(writer, "OK")
			}
		case "DEL":
			s.deletedKeys.Add(1)
			writeIntReply(writer, 1)
		case "SENTINEL":
			if s.sentinelMaster == "" {
				writeErrorReply(writer, "ERR unknown command")
				continue
			}
			host, port, _ := net.SplitHostPort(s.sentinelMaster)
			writeArrayReply(writer, host, port)
		default:
			writeErrorReply(writer, "ERR unknown command")
		}
	}
}

func writeStatusReply(writer *bufio.Writer, value string) {
	_, _ = fmt.Fprintf(writer, "+%s\r\n", value)
	_ = writer.Flush()
}

func writeErrorReply(writer *bufio.Writer, value string) {
	_, _ = fmt.Fprintf(writer, "-%s\r\n", value)
	_ = writer.Flush()
}

func writeBulkReply(writer *bufio.Writer, value string) {
	_, _ = fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(value), value)
	_ = writer.Flush()
}

func writeIntReply(writer *bufio.Writer, value int) {
	_, _ = fmt.Fprintf(writer, ":%d\r\n", value)
	_ = writer.Flush()
}

func writeArrayReply(writer *bufio.Writer, values ...string) {
	_, _ = fmt.Fprintf(writer, "*%d\r\n", len(values))
	for _, value := range values {
		_, _ = fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(value), value)
	}
	_ = writer.Flush()
}

func hasCheck(result connections.TestResult, code string, state connections.CheckState) bool {
	for _, item := range result.Checks {
		if item.Code == code && item.State == state {
			return true
		}
	}
	return false
}

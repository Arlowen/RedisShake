package client

import (
	"crypto/tls"
	"testing"
)

func TestGetTLSConfigPreservesLegacyAndSecureModes(t *testing.T) {
	legacy := getTlsConfig(TlsConfig{})
	if !legacy.InsecureSkipVerify {
		t.Fatal("legacy TLS config should preserve insecure_skip_verify=true when the field is omitted")
	}
	if legacy.MinVersion != tls.VersionTLS12 {
		t.Fatalf("legacy TLS minimum version = %d", legacy.MinVersion)
	}

	secure := false
	configured := getTlsConfig(TlsConfig{
		ServerName:         "redis.internal",
		InsecureSkipVerify: &secure,
	})
	if configured.InsecureSkipVerify {
		t.Fatal("explicit secure TLS config ignored insecure_skip_verify=false")
	}
	if configured.ServerName != "redis.internal" {
		t.Fatalf("ServerName = %q", configured.ServerName)
	}
}

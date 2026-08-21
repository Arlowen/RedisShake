package connections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"RedisShake/internal/controlplane/domain"
)

var ErrSecretsNotConfigured = errors.New("credential encryption is not configured")

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	ServerName         string `json:"server_name,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACertPEM          string `json:"ca_cert_pem,omitempty"`
	ClientCertPEM      string `json:"client_cert_pem,omitempty"`
	ClientKeyPEM       string `json:"client_key_pem,omitempty"`
}

type SentinelConfig struct {
	Address    string    `json:"address"`
	MasterName string    `json:"master_name"`
	Username   string    `json:"username,omitempty"`
	Password   string    `json:"password,omitempty"`
	TLS        TLSConfig `json:"tls"`
}

type Spec struct {
	Name     string          `json:"name"`
	Topology domain.Topology `json:"topology"`
	Address  string          `json:"address"`
	Username string          `json:"username,omitempty"`
	Password string          `json:"password,omitempty"`
	TLS      TLSConfig       `json:"tls"`
	Sentinel SentinelConfig  `json:"sentinel"`
}

type TLSPatch struct {
	Enabled            *bool   `json:"enabled,omitempty"`
	ServerName         *string `json:"server_name,omitempty"`
	InsecureSkipVerify *bool   `json:"insecure_skip_verify,omitempty"`
	CACertPEM          *string `json:"ca_cert_pem,omitempty"`
	ClientCertPEM      *string `json:"client_cert_pem,omitempty"`
	ClientKeyPEM       *string `json:"client_key_pem,omitempty"`
}

type SentinelPatch struct {
	Address    *string   `json:"address,omitempty"`
	MasterName *string   `json:"master_name,omitempty"`
	Username   *string   `json:"username,omitempty"`
	Password   *string   `json:"password,omitempty"`
	TLS        *TLSPatch `json:"tls,omitempty"`
}

type Patch struct {
	Name     *string          `json:"name,omitempty"`
	Topology *domain.Topology `json:"topology,omitempty"`
	Address  *string          `json:"address,omitempty"`
	Username *string          `json:"username,omitempty"`
	Password *string          `json:"password,omitempty"`
	TLS      *TLSPatch        `json:"tls,omitempty"`
	Sentinel *SentinelPatch   `json:"sentinel,omitempty"`
}

type TLSView struct {
	Enabled              bool   `json:"enabled"`
	ServerName           string `json:"server_name,omitempty"`
	InsecureSkipVerify   bool   `json:"insecure_skip_verify"`
	CACertConfigured     bool   `json:"ca_cert_configured"`
	ClientCertConfigured bool   `json:"client_cert_configured"`
	ClientKeyConfigured  bool   `json:"client_key_configured"`
}

type SentinelView struct {
	Address            string  `json:"address,omitempty"`
	MasterName         string  `json:"master_name,omitempty"`
	Username           string  `json:"username,omitempty"`
	PasswordConfigured bool    `json:"password_configured"`
	TLS                TLSView `json:"tls"`
}

type View struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Topology           domain.Topology `json:"topology"`
	Address            string          `json:"address,omitempty"`
	Username           string          `json:"username,omitempty"`
	PasswordConfigured bool            `json:"password_configured"`
	TLS                TLSView         `json:"tls"`
	Sentinel           SentinelView    `json:"sentinel"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	LastTestedAt       *time.Time      `json:"last_tested_at,omitempty"`
	LastTestResult     json.RawMessage `json:"last_test_result,omitempty"`
}

type TestPurpose string

const (
	TestPurposeSource TestPurpose = "source"
	TestPurposeTarget TestPurpose = "target"
)

func (p TestPurpose) Valid() bool {
	return p == TestPurposeSource || p == TestPurposeTarget
}

type CheckState string

const (
	CheckStatePass    CheckState = "PASS"
	CheckStateWarning CheckState = "WARNING"
	CheckStateFail    CheckState = "FAIL"
)

type CheckItem struct {
	Code    string     `json:"code"`
	State   CheckState `json:"state"`
	Message string     `json:"message"`
}

type TestResult struct {
	Success          bool        `json:"success"`
	Purpose          TestPurpose `json:"purpose"`
	EffectiveAddress string      `json:"effective_address,omitempty"`
	ServerProduct    string      `json:"server_product,omitempty"`
	ServerVersion    string      `json:"server_version,omitempty"`
	Role             string      `json:"role,omitempty"`
	ClusterEnabled   bool        `json:"cluster_enabled"`
	LatencyMillis    int64       `json:"latency_ms"`
	Checks           []CheckItem `json:"checks"`
	TestedAt         time.Time   `json:"tested_at"`
}

type Checker interface {
	Check(ctx context.Context, connection Resolved, purpose TestPurpose) TestResult
}

type Resolved struct {
	Spec
}

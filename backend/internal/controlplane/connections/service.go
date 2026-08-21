package connections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/ids"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
)

type Service struct {
	store   *store.Store
	cipher  *secrets.Cipher
	checker Checker
	now     func() time.Time
}

type storedTLSConfig struct {
	ServerName           string `json:"server_name,omitempty"`
	InsecureSkipVerify   bool   `json:"insecure_skip_verify"`
	CACertCiphertext     string `json:"ca_cert_ciphertext,omitempty"`
	ClientCertCiphertext string `json:"client_cert_ciphertext,omitempty"`
	ClientKeyCiphertext  string `json:"client_key_ciphertext,omitempty"`
}

func NewService(database *store.Store, cipher *secrets.Cipher, checker Checker) *Service {
	return &Service{
		store:   database,
		cipher:  cipher,
		checker: checker,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input Spec) (View, error) {
	input = normalizeSpec(input)
	if err := validateSpec(input); err != nil {
		return View{}, err
	}

	id, err := ids.New()
	if err != nil {
		return View{}, err
	}
	now := s.now()
	connection, err := s.toStored(input)
	if err != nil {
		return View{}, err
	}
	connection.ID = id
	connection.CreatedAt = now
	connection.UpdatedAt = now
	if err := s.store.CreateConnection(ctx, connection); err != nil {
		return View{}, err
	}
	return toView(connection), nil
}

func (s *Service) Get(ctx context.Context, id string) (View, error) {
	connection, err := s.store.GetConnection(ctx, id)
	if err != nil {
		return View{}, err
	}
	return toView(connection), nil
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	connections, err := s.store.ListConnections(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(connections))
	for _, connection := range connections {
		views = append(views, toView(connection))
	}
	return views, nil
}

func (s *Service) Update(ctx context.Context, id string, patch Patch) (View, error) {
	stored, err := s.store.GetConnection(ctx, id)
	if err != nil {
		return View{}, err
	}
	resolved, err := s.resolve(stored)
	if err != nil {
		return View{}, err
	}
	spec := resolved.Spec
	applyPatch(&spec, patch)
	spec = normalizeSpec(spec)
	if err := validateSpec(spec); err != nil {
		return View{}, err
	}

	updated, err := s.toStored(spec)
	if err != nil {
		return View{}, err
	}
	updated.ID = stored.ID
	updated.CreatedAt = stored.CreatedAt
	updated.UpdatedAt = s.now()
	// Any connection edit invalidates the previous test result.
	updated.LastTestedAt = nil
	updated.LastTestResultJSON = ""
	if err := s.store.UpdateConnection(ctx, updated); err != nil {
		return View{}, err
	}
	return toView(updated), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.DeleteConnection(ctx, id)
}

func (s *Service) TestUnsaved(ctx context.Context, input Spec, purpose TestPurpose) (TestResult, error) {
	if s.checker == nil {
		return TestResult{}, errors.New("Redis connection checker is not configured")
	}
	if !purpose.Valid() {
		return TestResult{}, &ValidationError{Field: "purpose", Message: "must be source or target"}
	}
	input = normalizeSpec(input)
	if err := validateSpec(input); err != nil {
		return TestResult{}, err
	}
	return s.checker.Check(ctx, Resolved{Spec: input}, purpose), nil
}

func (s *Service) TestSaved(ctx context.Context, id string, purpose TestPurpose) (TestResult, error) {
	if s.checker == nil {
		return TestResult{}, errors.New("Redis connection checker is not configured")
	}
	if !purpose.Valid() {
		return TestResult{}, &ValidationError{Field: "purpose", Message: "must be source or target"}
	}
	stored, err := s.store.GetConnection(ctx, id)
	if err != nil {
		return TestResult{}, err
	}
	resolved, err := s.resolve(stored)
	if err != nil {
		return TestResult{}, err
	}
	result := s.checker.Check(ctx, resolved, purpose)
	encoded, err := json.Marshal(result)
	if err != nil {
		return TestResult{}, fmt.Errorf("encode connection test result: %w", err)
	}
	if err := s.store.UpdateConnectionTestResult(ctx, id, result.TestedAt, string(encoded)); err != nil {
		return TestResult{}, err
	}
	return result, nil
}

func (s *Service) Resolve(ctx context.Context, id string) (Resolved, error) {
	stored, err := s.store.GetConnection(ctx, id)
	if err != nil {
		return Resolved{}, err
	}
	return s.resolve(stored)
}

func (s *Service) ValidateStoredSecrets(ctx context.Context) error {
	connections, err := s.store.ListConnections(ctx)
	if err != nil {
		return err
	}
	for _, connection := range connections {
		if _, err := s.resolve(connection); err != nil {
			return fmt.Errorf("validate encrypted connection %q: %w", connection.ID, err)
		}
	}
	return nil
}

func (s *Service) toStored(spec Spec) (domain.Connection, error) {
	passwordCiphertext, err := s.encrypt(spec.Password)
	if err != nil {
		return domain.Connection{}, err
	}
	tlsJSON, err := s.encodeTLS(spec.TLS)
	if err != nil {
		return domain.Connection{}, err
	}
	sentinelPasswordCiphertext, err := s.encrypt(spec.Sentinel.Password)
	if err != nil {
		return domain.Connection{}, err
	}
	sentinelTLSJSON, err := s.encodeTLS(spec.Sentinel.TLS)
	if err != nil {
		return domain.Connection{}, err
	}

	return domain.Connection{
		Name:                       spec.Name,
		Topology:                   spec.Topology,
		Address:                    spec.Address,
		Username:                   spec.Username,
		PasswordCiphertext:         passwordCiphertext,
		TLSEnabled:                 spec.TLS.Enabled,
		TLSConfigJSON:              tlsJSON,
		SentinelAddress:            spec.Sentinel.Address,
		SentinelMasterName:         spec.Sentinel.MasterName,
		SentinelUsername:           spec.Sentinel.Username,
		SentinelPasswordCiphertext: sentinelPasswordCiphertext,
		SentinelTLSEnabled:         spec.Sentinel.TLS.Enabled,
		SentinelTLSConfigJSON:      sentinelTLSJSON,
	}, nil
}

func (s *Service) resolve(connection domain.Connection) (Resolved, error) {
	password, err := s.decrypt(connection.PasswordCiphertext)
	if err != nil {
		return Resolved{}, err
	}
	tlsConfig, err := s.decodeTLS(connection.TLSEnabled, connection.TLSConfigJSON)
	if err != nil {
		return Resolved{}, err
	}
	sentinelPassword, err := s.decrypt(connection.SentinelPasswordCiphertext)
	if err != nil {
		return Resolved{}, err
	}
	sentinelTLSConfig, err := s.decodeTLS(connection.SentinelTLSEnabled, connection.SentinelTLSConfigJSON)
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{Spec: Spec{
		Name:     connection.Name,
		Topology: connection.Topology,
		Address:  connection.Address,
		Username: connection.Username,
		Password: password,
		TLS:      tlsConfig,
		Sentinel: SentinelConfig{
			Address:    connection.SentinelAddress,
			MasterName: connection.SentinelMasterName,
			Username:   connection.SentinelUsername,
			Password:   sentinelPassword,
			TLS:        sentinelTLSConfig,
		},
	}}, nil
}

func (s *Service) encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if s.cipher == nil {
		return "", ErrSecretsNotConfigured
	}
	return s.cipher.Encrypt(value)
}

func (s *Service) decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if s.cipher == nil {
		return "", ErrSecretsNotConfigured
	}
	return s.cipher.Decrypt(value)
}

func (s *Service) encodeTLS(config TLSConfig) (string, error) {
	stored := storedTLSConfig{
		ServerName:         config.ServerName,
		InsecureSkipVerify: config.InsecureSkipVerify,
	}
	var err error
	if stored.CACertCiphertext, err = s.encrypt(config.CACertPEM); err != nil {
		return "", err
	}
	if stored.ClientCertCiphertext, err = s.encrypt(config.ClientCertPEM); err != nil {
		return "", err
	}
	if stored.ClientKeyCiphertext, err = s.encrypt(config.ClientKeyPEM); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return "", fmt.Errorf("encode TLS settings: %w", err)
	}
	return string(encoded), nil
}

func (s *Service) decodeTLS(enabled bool, value string) (TLSConfig, error) {
	stored := storedTLSConfig{}
	if strings.TrimSpace(value) != "" {
		if err := json.Unmarshal([]byte(value), &stored); err != nil {
			return TLSConfig{}, fmt.Errorf("decode stored TLS settings: %w", err)
		}
	}
	caCert, err := s.decrypt(stored.CACertCiphertext)
	if err != nil {
		return TLSConfig{}, err
	}
	clientCert, err := s.decrypt(stored.ClientCertCiphertext)
	if err != nil {
		return TLSConfig{}, err
	}
	clientKey, err := s.decrypt(stored.ClientKeyCiphertext)
	if err != nil {
		return TLSConfig{}, err
	}
	return TLSConfig{
		Enabled:            enabled,
		ServerName:         stored.ServerName,
		InsecureSkipVerify: stored.InsecureSkipVerify,
		CACertPEM:          caCert,
		ClientCertPEM:      clientCert,
		ClientKeyPEM:       clientKey,
	}, nil
}

func toView(connection domain.Connection) View {
	tls := parseTLSView(connection.TLSEnabled, connection.TLSConfigJSON)
	sentinelTLS := parseTLSView(connection.SentinelTLSEnabled, connection.SentinelTLSConfigJSON)
	var lastResult json.RawMessage
	if json.Valid([]byte(connection.LastTestResultJSON)) {
		lastResult = json.RawMessage(connection.LastTestResultJSON)
	}
	return View{
		ID:                 connection.ID,
		Name:               connection.Name,
		Topology:           connection.Topology,
		Address:            connection.Address,
		Username:           connection.Username,
		PasswordConfigured: connection.PasswordCiphertext != "",
		TLS:                tls,
		Sentinel: SentinelView{
			Address:            connection.SentinelAddress,
			MasterName:         connection.SentinelMasterName,
			Username:           connection.SentinelUsername,
			PasswordConfigured: connection.SentinelPasswordCiphertext != "",
			TLS:                sentinelTLS,
		},
		CreatedAt:      connection.CreatedAt,
		UpdatedAt:      connection.UpdatedAt,
		LastTestedAt:   connection.LastTestedAt,
		LastTestResult: lastResult,
	}
}

func parseTLSView(enabled bool, value string) TLSView {
	stored := storedTLSConfig{}
	_ = json.Unmarshal([]byte(value), &stored)
	return TLSView{
		Enabled:              enabled,
		ServerName:           stored.ServerName,
		InsecureSkipVerify:   stored.InsecureSkipVerify,
		CACertConfigured:     stored.CACertCiphertext != "",
		ClientCertConfigured: stored.ClientCertCiphertext != "",
		ClientKeyConfigured:  stored.ClientKeyCiphertext != "",
	}
}

func normalizeSpec(spec Spec) Spec {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Address = strings.TrimSpace(spec.Address)
	spec.Username = strings.TrimSpace(spec.Username)
	spec.TLS.ServerName = strings.TrimSpace(spec.TLS.ServerName)
	spec.Sentinel.Address = strings.TrimSpace(spec.Sentinel.Address)
	spec.Sentinel.MasterName = strings.TrimSpace(spec.Sentinel.MasterName)
	spec.Sentinel.Username = strings.TrimSpace(spec.Sentinel.Username)
	spec.Sentinel.TLS.ServerName = strings.TrimSpace(spec.Sentinel.TLS.ServerName)
	if !spec.TLS.Enabled {
		spec.TLS = TLSConfig{}
	}
	if spec.Topology != domain.TopologySentinel {
		spec.Sentinel = SentinelConfig{}
	} else if !spec.Sentinel.TLS.Enabled {
		spec.Sentinel.TLS = TLSConfig{}
	}
	return spec
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return &ValidationError{Field: "name", Message: "is required"}
	}
	if len(spec.Name) > 128 {
		return &ValidationError{Field: "name", Message: "must be at most 128 characters"}
	}
	if !spec.Topology.Valid() {
		return &ValidationError{Field: "topology", Message: "must be standalone, sentinel, or cluster"}
	}
	if spec.Topology == domain.TopologySentinel {
		if spec.Address != "" {
			if err := validateAddress("address", spec.Address); err != nil {
				return err
			}
		}
		if err := validateAddress("sentinel.address", spec.Sentinel.Address); err != nil {
			return err
		}
		if spec.Sentinel.MasterName == "" {
			return &ValidationError{Field: "sentinel.master_name", Message: "is required for sentinel topology"}
		}
		if err := validateTLS("sentinel.tls", spec.Sentinel.TLS); err != nil {
			return err
		}
	} else if err := validateAddress("address", spec.Address); err != nil {
		return err
	}
	return validateTLS("tls", spec.TLS)
}

func validateAddress(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Message: "is required"}
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		return &ValidationError{Field: field, Message: "must use host:port format"}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return &ValidationError{Field: field, Message: "contains an invalid port"}
	}
	return nil
}

func validateTLS(field string, config TLSConfig) error {
	if !config.Enabled {
		return nil
	}
	if (config.ClientCertPEM == "") != (config.ClientKeyPEM == "") {
		return &ValidationError{Field: field, Message: "client certificate and client key must be configured together"}
	}
	return nil
}

func applyPatch(spec *Spec, patch Patch) {
	if patch.Name != nil {
		spec.Name = *patch.Name
	}
	if patch.Topology != nil {
		spec.Topology = *patch.Topology
	}
	if patch.Address != nil {
		spec.Address = *patch.Address
	}
	if patch.Username != nil {
		spec.Username = *patch.Username
	}
	if patch.Password != nil {
		spec.Password = *patch.Password
	}
	if patch.TLS != nil {
		applyTLSPatch(&spec.TLS, *patch.TLS)
	}
	if patch.Sentinel != nil {
		if patch.Sentinel.Address != nil {
			spec.Sentinel.Address = *patch.Sentinel.Address
		}
		if patch.Sentinel.MasterName != nil {
			spec.Sentinel.MasterName = *patch.Sentinel.MasterName
		}
		if patch.Sentinel.Username != nil {
			spec.Sentinel.Username = *patch.Sentinel.Username
		}
		if patch.Sentinel.Password != nil {
			spec.Sentinel.Password = *patch.Sentinel.Password
		}
		if patch.Sentinel.TLS != nil {
			applyTLSPatch(&spec.Sentinel.TLS, *patch.Sentinel.TLS)
		}
	}
}

func applyTLSPatch(config *TLSConfig, patch TLSPatch) {
	if patch.Enabled != nil {
		config.Enabled = *patch.Enabled
	}
	if patch.ServerName != nil {
		config.ServerName = *patch.ServerName
	}
	if patch.InsecureSkipVerify != nil {
		config.InsecureSkipVerify = *patch.InsecureSkipVerify
	}
	if patch.CACertPEM != nil {
		config.CACertPEM = *patch.CACertPEM
	}
	if patch.ClientCertPEM != nil {
		config.ClientCertPEM = *patch.ClientCertPEM
	}
	if patch.ClientKeyPEM != nil {
		config.ClientKeyPEM = *patch.ClientKeyPEM
	}
}

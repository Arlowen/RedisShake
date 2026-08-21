package redischeck

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"RedisShake/internal/client/proto"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/ids"
)

type Checker struct {
	Timeout time.Duration
}

type redisClient struct {
	connection net.Conn
	reader     *proto.Reader
	writer     *proto.Writer
	buffered   *bufio.Writer
}

type connectionError struct {
	code    string
	message string
	cause   error
}

func (e *connectionError) Error() string {
	return e.message
}

func (e *connectionError) Unwrap() error {
	return e.cause
}

func (c *Checker) Check(ctx context.Context, resolved connections.Resolved, purpose connections.TestPurpose) connections.TestResult {
	startedAt := time.Now()
	result := connections.TestResult{
		Purpose:  purpose,
		Checks:   make([]connections.CheckItem, 0, 8),
		TestedAt: startedAt.UTC(),
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	effectiveAddress := resolved.Address
	if resolved.Topology == domain.TopologySentinel {
		address, err := c.resolveSentinel(ctx, resolved.Sentinel)
		if err != nil {
			addFailure(&result, err)
			finish(&result, startedAt)
			return result
		}
		effectiveAddress = address
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "sentinel_resolution",
			State:   connections.CheckStatePass,
			Message: "Sentinel 已解析出 Redis 主节点",
		})
	}
	result.EffectiveAddress = effectiveAddress

	client, err := c.open(ctx, effectiveAddress, resolved.Username, resolved.Password, resolved.TLS)
	if err != nil {
		addFailure(&result, err)
		finish(&result, startedAt)
		return result
	}
	defer client.close()
	result.Checks = append(result.Checks,
		connections.CheckItem{Code: "connectivity", State: connections.CheckStatePass, Message: "TCP/TLS 连接成功"},
		connections.CheckItem{Code: "authentication", State: connections.CheckStatePass, Message: "Redis 认证成功"},
		connections.CheckItem{Code: "ping", State: connections.CheckStatePass, Message: "Redis PING 响应正常"},
	)
	if resolved.TLS.Enabled && resolved.TLS.InsecureSkipVerify {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "tls_verification_disabled",
			State:   connections.CheckStateWarning,
			Message: "TLS 证书校验已关闭，仅建议在受控测试环境使用",
		})
	}

	serverInfo, err := commandString(client, "INFO", "server")
	if err != nil {
		addFailure(&result, &connectionError{code: "server_info", message: "无法读取 Redis server 信息", cause: err})
		finish(&result, startedAt)
		return result
	}
	result.ServerProduct, result.ServerVersion = parseServerVersion(serverInfo)
	if result.ServerVersion == "" {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "server_version",
			State:   connections.CheckStateWarning,
			Message: "未能从 INFO server 识别 Redis/Valkey 版本",
		})
	} else {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "server_version",
			State:   connections.CheckStatePass,
			Message: fmt.Sprintf("已识别 %s %s", result.ServerProduct, result.ServerVersion),
		})
	}

	replicationInfo, err := commandString(client, "INFO", "replication")
	if err != nil {
		addFailure(&result, &connectionError{code: "replication_info", message: "无法读取 Redis replication 信息", cause: err})
		finish(&result, startedAt)
		return result
	}
	result.Role = parseInfoValue(replicationInfo, "role")
	result.Checks = append(result.Checks, connections.CheckItem{
		Code:    "replication_role",
		State:   connections.CheckStatePass,
		Message: fmt.Sprintf("Redis 节点角色：%s", fallback(result.Role, "unknown")),
	})

	clusterInfo, err := commandString(client, "INFO", "cluster")
	if err != nil {
		addFailure(&result, &connectionError{code: "cluster_info", message: "无法读取 Redis cluster 信息", cause: err})
		finish(&result, startedAt)
		return result
	}
	result.ClusterEnabled = parseInfoValue(clusterInfo, "cluster_enabled") == "1"
	expectsCluster := resolved.Topology == domain.TopologyCluster
	if result.ClusterEnabled != expectsCluster {
		message := "连接配置为非集群，但 Redis 返回 cluster_enabled=1"
		if expectsCluster {
			message = "连接配置为集群，但 Redis 返回 cluster_enabled=0"
		}
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "topology",
			State:   connections.CheckStateFail,
			Message: message,
		})
	} else {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "topology",
			State:   connections.CheckStatePass,
			Message: "Redis 拓扑与连接配置一致",
		})
	}

	if purpose == connections.TestPurposeTarget {
		c.checkTargetWrite(ctx, client, resolved, &result)
	}
	finish(&result, startedAt)
	return result
}

func (c *Checker) resolveSentinel(ctx context.Context, sentinel connections.SentinelConfig) (string, error) {
	client, err := c.open(ctx, sentinel.Address, sentinel.Username, sentinel.Password, sentinel.TLS)
	if err != nil {
		if typed := new(connectionError); errors.As(err, &typed) {
			typed.code = "sentinel_" + typed.code
			typed.message = "Sentinel " + typed.message
		}
		return "", err
	}
	defer client.close()
	reply, err := client.do("SENTINEL", "GET-MASTER-ADDR-BY-NAME", sentinel.MasterName)
	if err != nil {
		return "", &connectionError{code: "sentinel_resolution", message: "Sentinel 无法解析指定 master name", cause: err}
	}
	items, ok := reply.([]interface{})
	if !ok || len(items) != 2 {
		return "", &connectionError{code: "sentinel_resolution", message: "Sentinel 返回了无效的主节点地址"}
	}
	host, hostOK := items[0].(string)
	port, portOK := items[1].(string)
	if !hostOK || !portOK || host == "" || port == "" {
		return "", &connectionError{code: "sentinel_resolution", message: "Sentinel 返回了无效的主节点地址"}
	}
	return net.JoinHostPort(host, port), nil
}

func (c *Checker) open(ctx context.Context, address, username, password string, tlsSettings connections.TLSConfig) (*redisClient, error) {
	dialer := &net.Dialer{Timeout: c.timeout(), KeepAlive: 30 * time.Second}
	var connection net.Conn
	var err error
	if tlsSettings.Enabled {
		config, configErr := buildTLSConfig(address, tlsSettings)
		if configErr != nil {
			return nil, &connectionError{code: "tls_config", message: "TLS 配置无效", cause: configErr}
		}
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: config}
		connection, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		code := "connectivity"
		message := "无法连接 Redis 地址"
		if tlsSettings.Enabled {
			code = "tls_handshake"
			message = "Redis TLS 连接或证书校验失败"
		}
		return nil, &connectionError{code: code, message: message, cause: err}
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(c.timeout()))
	}

	bufferedReader := bufio.NewReader(connection)
	bufferedWriter := bufio.NewWriter(connection)
	client := &redisClient{
		connection: connection,
		reader:     proto.NewReader(bufferedReader),
		writer:     proto.NewWriter(bufferedWriter),
		buffered:   bufferedWriter,
	}
	if password != "" {
		var reply interface{}
		if username != "" {
			reply, err = client.do("AUTH", username, password)
		} else {
			reply, err = client.do("AUTH", password)
		}
		if err != nil || reply != "OK" {
			client.close()
			return nil, &connectionError{code: "authentication", message: "Redis 用户名或密码认证失败", cause: err}
		}
	}
	reply, err := client.do("PING")
	if err != nil || reply != "PONG" {
		client.close()
		return nil, &connectionError{code: "ping", message: "Redis PING 检查失败", cause: err}
	}
	return client, nil
}

func (c *Checker) checkTargetWrite(ctx context.Context, client *redisClient, resolved connections.Resolved, result *connections.TestResult) {
	id, err := ids.New()
	if err != nil {
		result.Checks = append(result.Checks, connections.CheckItem{Code: "target_write", State: connections.CheckStateFail, Message: "无法生成目标写权限测试 Key"})
		return
	}
	key := "__redisshake_ui_precheck:" + id
	writeClient := client
	reply, err := writeClient.do("SET", key, "1", "NX", "EX", 60)
	if redirectAddress, redirected := clusterRedirect(err); redirected && resolved.Topology == domain.TopologyCluster {
		redirectedClient, redirectErr := c.open(ctx, redirectAddress, resolved.Username, resolved.Password, resolved.TLS)
		if redirectErr != nil {
			result.Checks = append(result.Checks, connections.CheckItem{
				Code:    "cluster_redirect",
				State:   connections.CheckStateFail,
				Message: "Redis Cluster 返回的目标节点不可连接",
			})
			return
		}
		defer redirectedClient.close()
		writeClient = redirectedClient
		reply, err = writeClient.do("SET", key, "1", "NX", "EX", 60)
		if err == nil {
			result.Checks = append(result.Checks, connections.CheckItem{
				Code:    "cluster_redirect",
				State:   connections.CheckStatePass,
				Message: "Redis Cluster Key 路由节点可连接",
			})
		}
	}
	if err != nil || reply != "OK" {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "target_write",
			State:   connections.CheckStateFail,
			Message: "目标 Redis 临时 Key 写入失败，请检查写权限和只读状态",
		})
		return
	}
	result.Checks = append(result.Checks, connections.CheckItem{
		Code:    "target_write",
		State:   connections.CheckStatePass,
		Message: "目标 Redis 临时 Key 写入成功",
	})
	if _, err := writeClient.do("DEL", key); err != nil {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "target_cleanup",
			State:   connections.CheckStateWarning,
			Message: "临时测试 Key 删除失败，将在 60 秒 TTL 到期后自动清理",
		})
	} else {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "target_cleanup",
			State:   connections.CheckStatePass,
			Message: "目标 Redis 临时测试 Key 已清理",
		})
	}
}

func clusterRedirect(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	message := strings.Fields(err.Error())
	if len(message) != 3 || strings.ToUpper(message[0]) != "MOVED" {
		return "", false
	}
	if _, _, splitErr := net.SplitHostPort(message[2]); splitErr != nil {
		return "", false
	}
	return message[2], true
}

func (c *Checker) timeout() time.Duration {
	if c.Timeout <= 0 {
		return 5 * time.Second
	}
	return c.Timeout
}

func (c *redisClient) do(args ...interface{}) (interface{}, error) {
	if err := c.writer.WriteArgs(args); err != nil {
		return nil, err
	}
	if err := c.buffered.Flush(); err != nil {
		return nil, err
	}
	return c.reader.ReadReply()
}

func (c *redisClient) close() {
	if c != nil && c.connection != nil {
		_ = c.connection.Close()
	}
}

func commandString(client *redisClient, args ...interface{}) (string, error) {
	reply, err := client.do(args...)
	if err != nil {
		return "", err
	}
	value, ok := reply.(string)
	if !ok {
		return "", fmt.Errorf("unexpected Redis reply type %T", reply)
	}
	return value, nil
}

func buildTLSConfig(address string, settings connections.TLSConfig) (*tls.Config, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	serverName := settings.ServerName
	if serverName == "" {
		serverName = host
	}
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         serverName,
		InsecureSkipVerify: settings.InsecureSkipVerify,
	}
	if settings.CACertPEM != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(settings.CACertPEM)) {
			return nil, errors.New("CA certificate is not valid PEM")
		}
		config.RootCAs = pool
	}
	if settings.ClientCertPEM != "" || settings.ClientKeyPEM != "" {
		certificate, err := tls.X509KeyPair([]byte(settings.ClientCertPEM), []byte(settings.ClientKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}

func parseServerVersion(info string) (string, string) {
	if value := parseInfoValue(info, "valkey_version"); value != "" {
		return "Valkey", value
	}
	if value := parseInfoValue(info, "redis_version"); value != "" {
		return "Redis", value
	}
	return "", ""
}

func parseInfoValue(info, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func fallback(value, fallbackValue string) string {
	if value == "" {
		return fallbackValue
	}
	return value
}

func addFailure(result *connections.TestResult, err error) {
	item := connections.CheckItem{Code: "connection_test", State: connections.CheckStateFail, Message: "Redis 连接测试失败"}
	var typed *connectionError
	if errors.As(err, &typed) {
		item.Code = typed.code
		item.Message = typed.message
	}
	result.Checks = append(result.Checks, item)
}

func finish(result *connections.TestResult, startedAt time.Time) {
	result.LatencyMillis = time.Since(startedAt).Milliseconds()
	result.Success = true
	for _, item := range result.Checks {
		if item.State == connections.CheckStateFail {
			result.Success = false
			break
		}
	}
}

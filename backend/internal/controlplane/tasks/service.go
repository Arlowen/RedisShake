package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/ids"
	"RedisShake/internal/controlplane/store"
)

type Service struct {
	store       *store.Store
	connections *connections.Service
	renderer    Renderer
	runtimeDir  string
	now         func() time.Time
}

type readerEnvelope struct {
	Sync *SyncReaderOptions `json:"sync,omitempty"`
	Scan *ScanReaderOptions `json:"scan,omitempty"`
}

func NewService(database *store.Store, connectionService *connections.Service, renderer Renderer, runtimeDir string) *Service {
	return &Service{
		store:       database,
		connections: connectionService,
		renderer:    renderer,
		runtimeDir:  runtimeDir,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input Spec) (View, error) {
	input = normalizeSpec(input)
	if err := validateDraft(input); err != nil {
		return View{}, err
	}
	if err := s.validateConnectionReferences(ctx, input); err != nil {
		return View{}, err
	}
	id, err := ids.New()
	if err != nil {
		return View{}, err
	}
	now := s.now()
	stored, err := encodeTask(input)
	if err != nil {
		return View{}, err
	}
	stored.ID = id
	stored.State = domain.TaskStateDraft
	stored.ConfigRevision = 1
	stored.CreatedAt = now
	stored.UpdatedAt = now
	if err := s.store.CreateTask(ctx, stored); err != nil {
		return View{}, err
	}
	return decodeTask(stored)
}

func (s *Service) Get(ctx context.Context, id string) (View, error) {
	stored, err := s.store.GetTask(ctx, id)
	if err != nil {
		return View{}, err
	}
	return decodeTask(stored)
}

func (s *Service) List(ctx context.Context, includeArchived bool) ([]View, error) {
	storedTasks, err := s.store.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(storedTasks))
	for _, stored := range storedTasks {
		if stored.State == domain.TaskStateArchived && !includeArchived {
			continue
		}
		view, err := decodeTask(stored)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) Update(ctx context.Context, id string, patch Patch) (View, error) {
	if patch.ExpectedRevision <= 0 {
		return View{}, &ValidationError{Field: "expected_revision", Message: "must be greater than zero"}
	}
	stored, err := s.store.GetTask(ctx, id)
	if err != nil {
		return View{}, err
	}
	if stored.State == domain.TaskStateArchived {
		return View{}, ErrArchived
	}
	if stored.ConfigRevision != patch.ExpectedRevision {
		return View{}, store.ErrRevisionConflict
	}
	view, err := decodeTask(stored)
	if err != nil {
		return View{}, err
	}
	spec := view.Spec
	applyPatch(&spec, patch)
	spec = normalizeSpec(spec)
	if err := validateDraft(spec); err != nil {
		return View{}, err
	}
	if err := s.validateConnectionReferences(ctx, spec); err != nil {
		return View{}, err
	}
	updated, err := encodeTask(spec)
	if err != nil {
		return View{}, err
	}
	updated.ID = stored.ID
	updated.CreatedAt = stored.CreatedAt
	updated.UpdatedAt = s.now()
	newRevision, err := s.store.UpdateTask(ctx, updated, patch.ExpectedRevision)
	if err != nil {
		return View{}, err
	}
	updated.State = domain.TaskStateDraft
	updated.ConfigRevision = newRevision
	return decodeTask(updated)
}

func (s *Service) Archive(ctx context.Context, id string) error {
	return s.store.ArchiveTask(ctx, id, s.now())
}

func (s *Service) Copy(ctx context.Context, id, name string) (View, error) {
	stored, err := s.store.GetTask(ctx, id)
	if err != nil {
		return View{}, err
	}
	view, err := decodeTask(stored)
	if err != nil {
		return View{}, err
	}
	view.Spec.Name = strings.TrimSpace(name)
	if view.Spec.Name == "" {
		return View{}, &ValidationError{Field: "name", Message: "is required"}
	}
	return s.Create(ctx, view.Spec)
}

func (s *Service) validateConnectionReferences(ctx context.Context, spec Spec) error {
	for _, item := range []struct {
		field string
		id    string
	}{
		{field: "source_connection_id", id: spec.SourceConnectionID},
		{field: "target_connection_id", id: spec.TargetConnectionID},
	} {
		if item.id == "" {
			continue
		}
		if _, err := s.connections.Get(ctx, item.id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return &ValidationError{Field: item.field, Message: "references a connection that does not exist"}
			}
			return err
		}
	}
	return nil
}

func (s *Service) Precheck(ctx context.Context, id string, request PrecheckRequest) (PrecheckResult, error) {
	stored, err := s.store.GetTask(ctx, id)
	if err != nil {
		return PrecheckResult{}, err
	}
	if stored.State == domain.TaskStateArchived {
		return PrecheckResult{}, ErrArchived
	}
	if request.ExpectedRevision != stored.ConfigRevision {
		return PrecheckResult{}, store.ErrRevisionConflict
	}
	view, err := decodeTask(stored)
	if err != nil {
		return PrecheckResult{}, err
	}
	checkedAt := s.now()
	result := PrecheckResult{
		TaskID:         id,
		ConfigRevision: stored.ConfigRevision,
		Checks:         make([]connections.CheckItem, 0, 16),
		CheckedAt:      checkedAt,
	}
	appendSpecChecks(&result, view.Spec)

	var source, target connections.Resolved
	var sourceTest, targetTest connections.TestResult
	if !hasFailure(result.Checks) {
		sourceTest, err = s.connections.TestSaved(ctx, view.Spec.SourceConnectionID, connections.TestPurposeSource)
		if err != nil {
			return PrecheckResult{}, err
		}
		appendConnectionChecks(&result, "source", sourceTest)
		targetTest, err = s.connections.TestSaved(ctx, view.Spec.TargetConnectionID, connections.TestPurposeTarget)
		if err != nil {
			return PrecheckResult{}, err
		}
		appendConnectionChecks(&result, "target", targetTest)
		if sourceTest.Success && targetTest.Success && sourceTest.EffectiveAddress == targetTest.EffectiveAddress {
			result.Checks = append(result.Checks, failCheck("source_target_distinct", "源端和目标端解析到了同一个 Redis 地址"))
		}
		if sourceTest.Success && targetTest.Success {
			source, err = s.connections.Resolve(ctx, view.Spec.SourceConnectionID)
			if err != nil {
				return PrecheckResult{}, err
			}
			target, err = s.connections.Resolve(ctx, view.Spec.TargetConnectionID)
			if err != nil {
				return PrecheckResult{}, err
			}
			appendTopologySpecificChecks(&result, view.Spec, source, sourceTest)
		}
	}

	if !hasFailure(result.Checks) {
		if s.renderer == nil {
			return PrecheckResult{}, errors.New("RedisShake config renderer is not configured")
		}
		artifact, renderErr := s.renderer.Render(view.Spec, source, target, RuntimeConfig{
			RunDir: filepath.Join(s.runtimeDir, "tasks", id, "runs", "preview"),
		})
		if renderErr != nil {
			result.Checks = append(result.Checks, failCheck("config_generation", "RedisShake 配置生成或解析失败"))
		} else {
			digestMaterial := artifact.DigestMaterial
			if len(digestMaterial) == 0 {
				digestMaterial = artifact.TOML
			}
			digest := sha256.Sum256(digestMaterial)
			result.ConfigDigest = hex.EncodeToString(digest[:])
			result.Checks = append(result.Checks, passCheck("config_generation", "RedisShake 配置生成并通过内核解析"))
		}
	}

	if view.Spec.Advanced.EmptyDBBeforeSync {
		result.Checks = append(result.Checks, connections.CheckItem{
			Code:    "empty_target_database",
			State:   connections.CheckStateWarning,
			Message: "任务启动后会先清空目标 Redis 的全部数据库",
		})
	}
	hasWarnings := hasState(result.Checks, connections.CheckStateWarning)
	result.Ready = !hasFailure(result.Checks) && (!hasWarnings || request.AcknowledgeWarnings)
	encoded, err := json.Marshal(result)
	if err != nil {
		return PrecheckResult{}, fmt.Errorf("encode task precheck result: %w", err)
	}
	if result.Ready {
		err = s.store.MarkTaskReady(ctx, id, stored.ConfigRevision, checkedAt, string(encoded))
	} else {
		err = s.store.SaveTaskPrecheckResult(ctx, id, stored.ConfigRevision, checkedAt, string(encoded))
	}
	if err != nil {
		return PrecheckResult{}, err
	}
	return result, nil
}

func encodeTask(spec Spec) (domain.Task, error) {
	readerJSON, err := json.Marshal(readerEnvelope{Sync: spec.SyncReader, Scan: spec.ScanReader})
	if err != nil {
		return domain.Task{}, fmt.Errorf("encode reader options: %w", err)
	}
	filterJSON, err := json.Marshal(spec.Filter)
	if err != nil {
		return domain.Task{}, fmt.Errorf("encode filter options: %w", err)
	}
	advancedJSON, err := json.Marshal(spec.Advanced)
	if err != nil {
		return domain.Task{}, fmt.Errorf("encode advanced options: %w", err)
	}
	return domain.Task{
		Name:                spec.Name,
		Description:         spec.Description,
		Mode:                spec.Mode,
		SourceConnectionID:  spec.SourceConnectionID,
		TargetConnectionID:  spec.TargetConnectionID,
		ReaderOptionsJSON:   string(readerJSON),
		FilterOptionsJSON:   string(filterJSON),
		AdvancedOptionsJSON: string(advancedJSON),
	}, nil
}

func decodeTask(stored domain.Task) (View, error) {
	reader := readerEnvelope{}
	if err := json.Unmarshal([]byte(stored.ReaderOptionsJSON), &reader); err != nil {
		return View{}, fmt.Errorf("decode reader options: %w", err)
	}
	filter := FilterOptions{}
	if err := json.Unmarshal([]byte(stored.FilterOptionsJSON), &filter); err != nil {
		return View{}, fmt.Errorf("decode filter options: %w", err)
	}
	advanced := AdvancedOptions{}
	if err := json.Unmarshal([]byte(stored.AdvancedOptionsJSON), &advanced); err != nil {
		return View{}, fmt.Errorf("decode advanced options: %w", err)
	}
	var lastResult json.RawMessage
	if json.Valid([]byte(stored.LastPrecheckResultJSON)) {
		lastResult = json.RawMessage(stored.LastPrecheckResultJSON)
	}
	spec := normalizeSpec(Spec{
		Name:               stored.Name,
		Description:        stored.Description,
		Mode:               stored.Mode,
		SourceConnectionID: stored.SourceConnectionID,
		TargetConnectionID: stored.TargetConnectionID,
		SyncReader:         reader.Sync,
		ScanReader:         reader.Scan,
		Filter:             filter,
		Advanced:           advanced,
	})
	return View{
		ID:                 stored.ID,
		Spec:               spec,
		State:              stored.State,
		ConfigRevision:     stored.ConfigRevision,
		CreatedAt:          stored.CreatedAt,
		UpdatedAt:          stored.UpdatedAt,
		LastPrecheckedAt:   stored.LastPrecheckedAt,
		LastPrecheckResult: lastResult,
	}, nil
}

func normalizeSpec(spec Spec) Spec {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.SourceConnectionID = strings.TrimSpace(spec.SourceConnectionID)
	spec.TargetConnectionID = strings.TrimSpace(spec.TargetConnectionID)
	if spec.Mode == domain.TaskModeSync {
		if spec.SyncReader == nil {
			spec.SyncReader = &SyncReaderOptions{SyncRDB: true, SyncAOF: true}
		}
		spec.ScanReader = nil
	}
	if spec.Mode == domain.TaskModeScan {
		if spec.ScanReader == nil {
			spec.ScanReader = &ScanReaderOptions{Scan: true, Count: 1}
		}
		spec.SyncReader = nil
		spec.ScanReader.DBs = nonNilIntSlice(spec.ScanReader.DBs)
		spec.ScanReader.SkipUnknownType = nonNilStringSlice(spec.ScanReader.SkipUnknownType)
	}
	spec.Filter.AllowKeys = nonNilStringSlice(spec.Filter.AllowKeys)
	spec.Filter.AllowKeyPrefix = nonNilStringSlice(spec.Filter.AllowKeyPrefix)
	spec.Filter.AllowKeySuffix = nonNilStringSlice(spec.Filter.AllowKeySuffix)
	spec.Filter.AllowKeyRegex = nonNilStringSlice(spec.Filter.AllowKeyRegex)
	spec.Filter.BlockKeys = nonNilStringSlice(spec.Filter.BlockKeys)
	spec.Filter.BlockKeyPrefix = nonNilStringSlice(spec.Filter.BlockKeyPrefix)
	spec.Filter.BlockKeySuffix = nonNilStringSlice(spec.Filter.BlockKeySuffix)
	spec.Filter.BlockKeyRegex = nonNilStringSlice(spec.Filter.BlockKeyRegex)
	spec.Filter.AllowDB = nonNilIntSlice(spec.Filter.AllowDB)
	spec.Filter.BlockDB = nonNilIntSlice(spec.Filter.BlockDB)
	spec.Filter.AllowCommand = nonNilStringSlice(spec.Filter.AllowCommand)
	spec.Filter.BlockCommand = nonNilStringSlice(spec.Filter.BlockCommand)
	spec.Filter.AllowCommandGroup = nonNilStringSlice(spec.Filter.AllowCommandGroup)
	spec.Filter.BlockCommandGroup = nonNilStringSlice(spec.Filter.BlockCommandGroup)
	if spec.Advanced.RDBRestoreCommandBehavior == "" {
		spec.Advanced.RDBRestoreCommandBehavior = "panic"
	}
	if spec.Advanced.PipelineCountLimit == 0 {
		spec.Advanced.PipelineCountLimit = 1024
	}
	if spec.Advanced.TargetRedisMaxQPS == 0 {
		spec.Advanced.TargetRedisMaxQPS = 300000
	}
	if spec.Advanced.TargetRedisClientMaxQuerybuf == 0 {
		spec.Advanced.TargetRedisClientMaxQuerybuf = 1073741824
	}
	if spec.Advanced.TargetRedisProtoMaxBulkLen == 0 {
		spec.Advanced.TargetRedisProtoMaxBulkLen = 512000000
	}
	return spec
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilIntSlice(values []int) []int {
	if values == nil {
		return []int{}
	}
	return values
}

func validateDraft(spec Spec) error {
	if spec.Name == "" {
		return &ValidationError{Field: "name", Message: "is required"}
	}
	if len(spec.Name) > 128 {
		return &ValidationError{Field: "name", Message: "must be at most 128 characters"}
	}
	if !spec.Mode.Valid() {
		return &ValidationError{Field: "mode", Message: "must be sync or scan"}
	}
	return nil
}

func appendSpecChecks(result *PrecheckResult, spec Spec) {
	if spec.SourceConnectionID == "" {
		result.Checks = append(result.Checks, failCheck("source_connection", "请选择源端 Redis 连接"))
	}
	if spec.TargetConnectionID == "" {
		result.Checks = append(result.Checks, failCheck("target_connection", "请选择目标端 Redis 连接"))
	}
	if spec.SourceConnectionID != "" && spec.SourceConnectionID == spec.TargetConnectionID {
		result.Checks = append(result.Checks, failCheck("source_target_distinct", "源端和目标端不能使用同一个连接"))
	}
	if spec.Mode == domain.TaskModeSync {
		if spec.SyncReader == nil || (!spec.SyncReader.SyncRDB && !spec.SyncReader.SyncAOF) {
			result.Checks = append(result.Checks, failCheck("sync_reader", "sync 模式至少需要启用 RDB 或 AOF 阶段"))
		}
	}
	if spec.Mode == domain.TaskModeScan {
		if spec.ScanReader == nil || (!spec.ScanReader.Scan && !spec.ScanReader.KSN) {
			result.Checks = append(result.Checks, failCheck("scan_reader", "scan 模式至少需要启用 Scan 或 KSN"))
		} else if spec.ScanReader.Scan && spec.ScanReader.Count <= 0 {
			result.Checks = append(result.Checks, failCheck("scan_count", "Scan count 必须大于 0"))
		}
	}
	for _, pair := range []struct {
		code  string
		allow int
		block int
	}{
		{code: "filter_keys", allow: len(spec.Filter.AllowKeys) + len(spec.Filter.AllowKeyPrefix) + len(spec.Filter.AllowKeySuffix) + len(spec.Filter.AllowKeyRegex), block: len(spec.Filter.BlockKeys) + len(spec.Filter.BlockKeyPrefix) + len(spec.Filter.BlockKeySuffix) + len(spec.Filter.BlockKeyRegex)},
		{code: "filter_db", allow: len(spec.Filter.AllowDB), block: len(spec.Filter.BlockDB)},
		{code: "filter_command", allow: len(spec.Filter.AllowCommand) + len(spec.Filter.AllowCommandGroup), block: len(spec.Filter.BlockCommand) + len(spec.Filter.BlockCommandGroup)},
	} {
		if pair.allow > 0 && pair.block > 0 {
			result.Checks = append(result.Checks, failCheck(pair.code, "同一过滤维度不能同时配置 allow 和 block 规则"))
		}
	}
	for _, expression := range append(append([]string{}, spec.Filter.AllowKeyRegex...), spec.Filter.BlockKeyRegex...) {
		if _, err := regexp.Compile(expression); err != nil {
			result.Checks = append(result.Checks, failCheck("filter_regex", "Key 正则表达式无效"))
			break
		}
	}
	databaseIDs := append(append([]int{}, spec.Filter.AllowDB...), spec.Filter.BlockDB...)
	if spec.ScanReader != nil {
		databaseIDs = append(databaseIDs, spec.ScanReader.DBs...)
	}
	for _, databaseID := range databaseIDs {
		if databaseID < 0 {
			result.Checks = append(result.Checks, failCheck("database_id", "Redis DB 编号不能为负数"))
			break
		}
	}
	if behavior := spec.Advanced.RDBRestoreCommandBehavior; behavior != "panic" && behavior != "rewrite" && behavior != "skip" {
		result.Checks = append(result.Checks, failCheck("restore_behavior", "RDB 冲突处理必须是 panic、rewrite 或 skip"))
	}
	if spec.Advanced.TargetRedisMaxQPS < 1 || spec.Advanced.TargetRedisMaxQPS > 300000 {
		result.Checks = append(result.Checks, failCheck("target_qps", "目标 Redis 最大 QPS 必须在 1 到 300000 之间"))
	}
	if spec.Advanced.TargetRedisProtoMaxBulkLen < 1024*1024 {
		result.Checks = append(result.Checks, failCheck("proto_max_bulk_len", "proto max bulk len 不能小于 1 MiB"))
	}
	if !hasFailure(result.Checks) {
		result.Checks = append(result.Checks, passCheck("task_config", "任务字段与过滤配置校验通过"))
	}
}

func appendConnectionChecks(result *PrecheckResult, prefix string, connectionResult connections.TestResult) {
	for _, item := range connectionResult.Checks {
		item.Code = prefix + "." + item.Code
		if prefix == "source" {
			item.Message = "源端：" + item.Message
		} else {
			item.Message = "目标端：" + item.Message
		}
		result.Checks = append(result.Checks, item)
	}
}

func appendTopologySpecificChecks(result *PrecheckResult, spec Spec, source connections.Resolved, sourceTest connections.TestResult) {
	if spec.Mode == domain.TaskModeSync && sourceTest.ServerVersion != "" && versionLessThan(sourceTest.ServerVersion, 2, 8) {
		result.Checks = append(result.Checks, failCheck("source_version", "sync 模式要求源端 Redis 版本不低于 2.8"))
	}
	if source.Topology == domain.TopologyCluster {
		for _, db := range append(append([]int{}, spec.Filter.AllowDB...), spec.Filter.BlockDB...) {
			if db != 0 {
				result.Checks = append(result.Checks, failCheck("cluster_db", "Redis Cluster 只支持 DB 0"))
				break
			}
		}
		if spec.ScanReader != nil {
			for _, db := range spec.ScanReader.DBs {
				if db != 0 {
					result.Checks = append(result.Checks, failCheck("cluster_db", "Redis Cluster 只支持 DB 0"))
					break
				}
			}
		}
	}
}

func versionLessThan(version string, major, minor int) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	parsedMajor, majorErr := strconv.Atoi(parts[0])
	parsedMinor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil {
		return false
	}
	return parsedMajor < major || (parsedMajor == major && parsedMinor < minor)
}

func applyPatch(spec *Spec, patch Patch) {
	if patch.Name != nil {
		spec.Name = *patch.Name
	}
	if patch.Description != nil {
		spec.Description = *patch.Description
	}
	if patch.Mode != nil {
		spec.Mode = *patch.Mode
	}
	if patch.SourceConnectionID != nil {
		spec.SourceConnectionID = *patch.SourceConnectionID
	}
	if patch.TargetConnectionID != nil {
		spec.TargetConnectionID = *patch.TargetConnectionID
	}
	if patch.SyncReader != nil {
		value := *patch.SyncReader
		spec.SyncReader = &value
	}
	if patch.ScanReader != nil {
		value := *patch.ScanReader
		spec.ScanReader = &value
	}
	if patch.Filter != nil {
		spec.Filter = *patch.Filter
	}
	if patch.Advanced != nil {
		spec.Advanced = *patch.Advanced
	}
}

func hasFailure(checks []connections.CheckItem) bool {
	return hasState(checks, connections.CheckStateFail)
}

func hasState(checks []connections.CheckItem, state connections.CheckState) bool {
	for _, item := range checks {
		if item.State == state {
			return true
		}
	}
	return false
}

func passCheck(code, message string) connections.CheckItem {
	return connections.CheckItem{Code: code, State: connections.CheckStatePass, Message: message}
}

func failCheck(code, message string) connections.CheckItem {
	return connections.CheckItem{Code: code, State: connections.CheckStateFail, Message: message}
}

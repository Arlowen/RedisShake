package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/ids"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/tasks"
)

type ManagerConfig struct {
	WorkerPath   string
	RuntimeDir   string
	StartTimeout time.Duration
	StopTimeout  time.Duration
}

type Manager struct {
	store       *store.Store
	tasks       *tasks.Service
	connections *connections.Service
	renderer    tasks.Renderer
	config      ManagerConfig
	httpClient  *http.Client
	now         func() time.Time

	mutex         sync.Mutex
	processes     map[string]*managedProcess
	workerOnce    sync.Once
	workerVersion string
	workerErr     error
}

type managedProcess struct {
	runID     string
	mode      domain.TaskMode
	command   *exec.Cmd
	done      chan exitResult
	logFile   *os.File
	finalOnce sync.Once

	mutex          sync.Mutex
	expectedStop   bool
	stopReason     string
	startupFailure string
}

type exitResult struct {
	err      error
	exitCode int
}

type processMetadata struct {
	RunID            string    `json:"run_id"`
	PID              int       `json:"pid"`
	WorkerPath       string    `json:"worker_path"`
	ProcessStartedAt time.Time `json:"process_started_at"`
	StatusAddress    string    `json:"status_address"`
}

type redactingWriter struct {
	mutex sync.Mutex
	file  *os.File
}

func NewManager(database *store.Store, taskService *tasks.Service, connectionService *connections.Service, renderer tasks.Renderer, config ManagerConfig) *Manager {
	if config.StartTimeout <= 0 {
		config.StartTimeout = 15 * time.Second
	}
	if config.StopTimeout <= 0 {
		config.StopTimeout = 30 * time.Second
	}
	return &Manager{
		store:       database,
		tasks:       taskService,
		connections: connectionService,
		renderer:    renderer,
		config:      config,
		httpClient:  &http.Client{Timeout: 2 * time.Second},
		now:         func() time.Time { return time.Now().UTC() },
		processes:   make(map[string]*managedProcess),
	}
}

func (m *Manager) Initialize(ctx context.Context) (int64, error) {
	return m.store.MarkActiveRunsUnknown(ctx, "control plane restarted; worker ownership could not be proven", m.now())
}

func (m *Manager) Start(ctx context.Context, taskID string, request StartRequest) (RunView, error) {
	if request.ExpectedRevision <= 0 {
		return RunView{}, store.ErrRevisionConflict
	}
	if _, err := m.VerifyWorker(ctx); err != nil {
		return RunView{}, err
	}
	taskView, err := m.tasks.Get(ctx, taskID)
	if err != nil {
		return RunView{}, err
	}
	if taskView.State != domain.TaskStateReady || taskView.ConfigRevision != request.ExpectedRevision {
		return RunView{}, store.ErrTaskNotReady
	}
	source, err := m.connections.Resolve(ctx, taskView.Spec.SourceConnectionID)
	if err != nil {
		return RunView{}, err
	}
	target, err := m.connections.Resolve(ctx, taskView.Spec.TargetConnectionID)
	if err != nil {
		return RunView{}, err
	}
	runID, err := ids.New()
	if err != nil {
		return RunView{}, err
	}
	statusPort, err := allocateStatusPort()
	if err != nil {
		return RunView{}, err
	}
	runDir := filepath.Join(m.config.RuntimeDir, "tasks", taskID, "runs", runID)
	artifact, err := m.renderer.Render(taskView.Spec, source, target, tasks.RuntimeConfig{RunDir: runDir, StatusPort: statusPort})
	if err != nil {
		return RunView{}, fmt.Errorf("render worker config: %w", err)
	}
	snapshot, err := json.Marshal(taskView.Spec)
	if err != nil {
		return RunView{}, fmt.Errorf("encode run config snapshot: %w", err)
	}
	startedAt := m.now()
	run := domain.Run{
		ID:                 runID,
		TaskID:             taskID,
		ConfigRevision:     taskView.ConfigRevision,
		ConfigSnapshotJSON: string(snapshot),
		State:              domain.RunStateStarting,
		RuntimeDir:         runDir,
		StartedAt:          startedAt,
		UpdatedAt:          startedAt,
	}
	if err := m.store.CreateRunIfNoActive(ctx, run); err != nil {
		return RunView{}, err
	}
	configPath, stdoutPath, err := materializeArtifact(runDir, artifact)
	if err != nil {
		m.finalizeStartFailure(runID, err)
		view, _ := m.Get(ctx, runID)
		return view, fmt.Errorf("%w: %v", ErrWorkerStart, err)
	}
	logFile, err := os.OpenFile(stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		m.finalizeStartFailure(runID, err)
		view, _ := m.Get(ctx, runID)
		return view, fmt.Errorf("%w: %v", ErrWorkerStart, err)
	}
	command := exec.Command(m.config.WorkerPath, configPath)
	command.Dir = runDir
	writer := &redactingWriter{file: logFile}
	command.Stdout = writer
	command.Stderr = writer
	configureCommand(command)
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		m.finalizeStartFailure(runID, err)
		view, _ := m.Get(ctx, runID)
		return view, fmt.Errorf("%w: %v", ErrWorkerStart, err)
	}
	process := &managedProcess{
		runID:   runID,
		mode:    taskView.Spec.Mode,
		command: command,
		done:    make(chan exitResult, 1),
		logFile: logFile,
	}
	m.mutex.Lock()
	m.processes[runID] = process
	m.mutex.Unlock()
	go waitForProcess(process)
	processStartedAt := m.now()
	if err := m.store.UpdateRunStarted(context.Background(), runID, command.Process.Pid, processStartedAt, statusPort, m.config.WorkerPath); err != nil {
		process.setStartupFailure("failed to persist RedisShake process metadata")
		_ = signalForce(command)
		go m.monitor(process, statusPort)
		view, _ := m.Get(context.Background(), runID)
		return view, err
	}
	if err := writeProcessMetadata(runDir, processMetadata{
		RunID:            runID,
		PID:              command.Process.Pid,
		WorkerPath:       m.config.WorkerPath,
		ProcessStartedAt: processStartedAt,
		StatusAddress:    statusURL(statusPort),
	}); err != nil {
		process.setStartupFailure("failed to write RedisShake process metadata")
		_ = signalForce(command)
		go m.monitor(process, statusPort)
		view, _ := m.Get(context.Background(), runID)
		return view, err
	}

	view, err := m.waitUntilStarted(ctx, process, statusPort)
	if err != nil {
		return view, err
	}
	return view, nil
}

func (m *Manager) VerifyWorker(ctx context.Context) (string, error) {
	m.workerOnce.Do(func() {
		workerInfo, err := os.Stat(m.config.WorkerPath)
		if err != nil || workerInfo.IsDir() {
			m.workerErr = fmt.Errorf("%w: %s", ErrWorkerUnavailable, m.config.WorkerPath)
			return
		}
		versionContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		command := exec.CommandContext(versionContext, m.config.WorkerPath, "--version")
		output, err := command.CombinedOutput()
		if err != nil {
			m.workerErr = fmt.Errorf("%w: version probe failed", ErrWorkerUnavailable)
			return
		}
		version := strings.TrimSpace(string(output))
		if !strings.HasPrefix(version, "redis-shake version ") {
			m.workerErr = fmt.Errorf("%w: unexpected version output", ErrWorkerUnavailable)
			return
		}
		m.workerVersion = version
	})
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return m.workerVersion, m.workerErr
}

func (m *Manager) waitUntilStarted(ctx context.Context, process *managedProcess, statusPort int) (RunView, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(m.config.StartTimeout)
	defer timeout.Stop()
	for {
		select {
		case result := <-process.done:
			m.handleExit(process, result)
			view, _ := m.Get(context.Background(), process.runID)
			if view.State == domain.RunStateSucceeded {
				return view, nil
			}
			return view, ErrWorkerStart
		case <-ticker.C:
			status, err := m.fetchStatus(ctx, statusPort)
			if err != nil {
				continue
			}
			now := m.now()
			if err := m.store.MarkRunRunning(context.Background(), process.runID, status, now); err != nil {
				go m.monitor(process, statusPort)
				view, _ := m.Get(context.Background(), process.runID)
				return view, err
			}
			go m.monitor(process, statusPort)
			return m.Get(context.Background(), process.runID)
		case <-timeout.C:
			process.setStartupFailure("RedisShake status endpoint did not become ready before the startup timeout")
			_ = m.store.MarkRunStatusUnhealthy(context.Background(), process.runID, m.now())
			_ = signalGraceful(process.command)
			go m.monitor(process, statusPort)
			view, _ := m.Get(context.Background(), process.runID)
			return view, fmt.Errorf("%w: status endpoint timeout", ErrWorkerStart)
		case <-ctx.Done():
			process.setStartupFailure("run start request was canceled")
			_ = signalGraceful(process.command)
			go m.monitor(process, statusPort)
			return RunView{}, ctx.Err()
		}
	}
}

func (m *Manager) monitor(process *managedProcess, statusPort int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case result := <-process.done:
			m.handleExit(process, result)
			return
		case <-ticker.C:
			status, err := m.fetchStatus(context.Background(), statusPort)
			if err != nil {
				failures++
				if failures >= 3 {
					_ = m.store.MarkRunStatusUnhealthy(context.Background(), process.runID, m.now())
				}
				continue
			}
			failures = 0
			_ = m.store.UpdateRunStatus(context.Background(), process.runID, status, m.now())
		}
	}
}

func (m *Manager) Get(ctx context.Context, id string) (RunView, error) {
	run, err := m.store.GetRun(ctx, id)
	if err != nil {
		return RunView{}, err
	}
	return toRunView(run), nil
}

func (m *Manager) ListByTask(ctx context.Context, taskID string) ([]RunView, error) {
	runs, err := m.store.ListRunsByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	views := make([]RunView, 0, len(runs))
	for _, run := range runs {
		views = append(views, toRunView(run))
	}
	return views, nil
}

func (m *Manager) Stop(ctx context.Context, id string) (RunView, error) {
	process := m.managed(id)
	if process == nil {
		run, err := m.store.GetRun(ctx, id)
		if err != nil {
			return RunView{}, err
		}
		if isTerminal(run.State) {
			return RunView{}, store.ErrConflict
		}
		return RunView{}, ErrRunNotManaged
	}
	if err := m.store.RequestRunStop(ctx, id, m.now()); err != nil {
		return RunView{}, err
	}
	process.setExpectedStop("stopped by user")
	if err := signalGraceful(process.command); err != nil {
		return RunView{}, fmt.Errorf("signal RedisShake worker: %w", err)
	}
	return m.Get(ctx, id)
}

func (m *Manager) ForceStop(ctx context.Context, id string) (RunView, error) {
	process := m.managed(id)
	if process == nil {
		run, err := m.store.GetRun(ctx, id)
		if err != nil {
			return RunView{}, err
		}
		if isTerminal(run.State) {
			return RunView{}, store.ErrConflict
		}
		return RunView{}, ErrRunNotManaged
	}
	if err := m.store.RequestRunStop(ctx, id, m.now()); err != nil {
		return RunView{}, err
	}
	process.setExpectedStop("force-stopped by user")
	if err := signalForce(process.command); err != nil {
		return RunView{}, fmt.Errorf("kill RedisShake worker: %w", err)
	}
	return m.Get(ctx, id)
}

func isTerminal(state domain.RunState) bool {
	return state == domain.RunStateStopped || state == domain.RunStateSucceeded || state == domain.RunStateFailed
}

func (m *Manager) ReadLogs(ctx context.Context, id string, offset int64, limit int) (LogResult, error) {
	if offset < 0 {
		return LogResult{}, errors.New("log offset cannot be negative")
	}
	if limit <= 0 {
		limit = 64 * 1024
	}
	if limit > 1024*1024 {
		limit = 1024 * 1024
	}
	run, err := m.store.GetRun(ctx, id)
	if err != nil {
		return LogResult{}, err
	}
	file, err := os.Open(filepath.Join(run.RuntimeDir, stdoutFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return LogResult{NextOffset: offset, EOF: true}, nil
		}
		return LogResult{}, fmt.Errorf("open run log: %w", err)
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return LogResult{}, fmt.Errorf("seek run log: %w", err)
	}
	buffer := make([]byte, limit)
	count, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return LogResult{}, fmt.Errorf("read run log: %w", err)
	}
	nextOffset := offset + int64(count)
	info, statErr := file.Stat()
	if statErr != nil {
		return LogResult{}, fmt.Errorf("stat run log: %w", statErr)
	}
	return LogResult{
		Content:    secrets.Redact(string(buffer[:count])),
		NextOffset: nextOffset,
		EOF:        nextOffset >= info.Size(),
	}, nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mutex.Lock()
	processes := make([]*managedProcess, 0, len(m.processes))
	for _, process := range m.processes {
		processes = append(processes, process)
	}
	m.mutex.Unlock()
	for _, process := range processes {
		_ = m.store.RequestRunStop(context.Background(), process.runID, m.now())
		process.setExpectedStop("stopped during control-plane shutdown")
		_ = signalGraceful(process.command)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(m.config.StopTimeout)
	defer timeout.Stop()
	for {
		if m.managedCount() == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			m.forceAll()
			if m.waitForManaged(3 * time.Second) {
				return nil
			}
			return ctx.Err()
		case <-timeout.C:
			m.forceAll()
			if m.waitForManaged(3 * time.Second) {
				return nil
			}
			return errors.New("timed out waiting for RedisShake workers to stop")
		case <-ticker.C:
		}
	}
}

func (m *Manager) waitForManaged(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.managedCount() == 0 {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return m.managedCount() == 0
}

func (m *Manager) fetchStatus(ctx context.Context, port int) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL(port), nil)
	if err != nil {
		return "", err
	}
	response, err := m.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status endpoint returned %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	if !json.Valid(contents) {
		return "", errors.New("status endpoint returned invalid JSON")
	}
	buffer := new(bytes.Buffer)
	if err := json.Compact(buffer, contents); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (m *Manager) finalizeStartFailure(runID string, cause error) {
	now := m.now()
	exitCode := -1
	_ = m.store.FinalizeRun(context.Background(), runID, domain.RunStateFailed, &exitCode, secrets.Redact(cause.Error()), now)
}

func (m *Manager) handleExit(process *managedProcess, result exitResult) {
	process.finalOnce.Do(func() {
		_ = process.logFile.Sync()
		_ = process.logFile.Close()
		expectedStop, stopReason, startupFailure := process.outcome()
		if stored, err := m.store.GetRun(context.Background(), process.runID); err == nil && stored.StopRequestedByUser {
			expectedStop = true
			if stopReason == "" {
				stopReason = "stopped by user"
			}
		}
		state := domain.RunStateFailed
		reason := "RedisShake worker exited unexpectedly"
		if startupFailure != "" {
			reason = startupFailure
		} else if expectedStop {
			state = domain.RunStateStopped
			reason = stopReason
		} else if process.mode == domain.TaskModeScan && result.exitCode == 0 {
			state = domain.RunStateSucceeded
			reason = "scan completed"
		} else if result.err != nil {
			reason = result.err.Error()
		}
		reason = secrets.Redact(reason)
		_ = m.store.FinalizeRun(context.Background(), process.runID, state, &result.exitCode, reason, m.now())
		m.mutex.Lock()
		delete(m.processes, process.runID)
		m.mutex.Unlock()
	})
}

func (m *Manager) managed(id string) *managedProcess {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.processes[id]
}

func (m *Manager) managedCount() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return len(m.processes)
}

func (m *Manager) forceAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, process := range m.processes {
		_ = signalForce(process.command)
	}
}

func (p *managedProcess) setExpectedStop(reason string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.expectedStop = true
	p.stopReason = reason
}

func (p *managedProcess) setStartupFailure(reason string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.startupFailure = reason
}

func (p *managedProcess) outcome() (bool, string, string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.expectedStop, p.stopReason, p.startupFailure
}

func waitForProcess(process *managedProcess) {
	err := process.command.Wait()
	exitCode := -1
	if process.command.ProcessState != nil {
		exitCode = process.command.ProcessState.ExitCode()
	}
	process.done <- exitResult{err: err, exitCode: exitCode}
}

func allocateStatusPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate status port: %w", err)
	}
	defer listener.Close()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, err
	}
	return port, nil
}

func statusURL(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port)
}

func (w *redactingWriter) Write(contents []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	redacted := []byte(secrets.Redact(string(contents)))
	if _, err := w.file.Write(redacted); err != nil {
		return 0, err
	}
	return len(contents), nil
}

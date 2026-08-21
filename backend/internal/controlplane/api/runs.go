package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/engine"
	"RedisShake/internal/controlplane/store"
)

func (s *Server) handleStartRun(w http.ResponseWriter, request *http.Request) {
	var input engine.StartRequest
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	run, err := s.engine.Start(request.Context(), request.PathValue("id"), input)
	if err != nil {
		if run.ID != "" {
			s.writeRunStartError(w, run, err)
		} else {
			s.writeRunError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) handleListRuns(w http.ResponseWriter, request *http.Request) {
	runs, err := s.engine.ListByTask(request.Context(), request.PathValue("id"))
	if err != nil {
		s.writeRunError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs})
}

func (s *Server) handleGetRun(w http.ResponseWriter, request *http.Request) {
	run, err := s.engine.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		s.writeRunError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleStopRun(w http.ResponseWriter, request *http.Request) {
	run, err := s.engine.Stop(request.Context(), request.PathValue("id"))
	if err != nil {
		s.writeRunError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleForceStopRun(w http.ResponseWriter, request *http.Request) {
	run, err := s.engine.ForceStop(request.Context(), request.PathValue("id"))
	if err != nil {
		s.writeRunError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleRunLogs(w http.ResponseWriter, request *http.Request) {
	offset, err := parseIntegerQuery(request, "offset", 0)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "offset must be a non-negative integer")
		return
	}
	limit, err := parseIntegerQuery(request, "limit", 65536)
	if err != nil || limit <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
		return
	}
	logs, err := s.engine.ReadLogs(request.Context(), request.PathValue("id"), int64(offset), limit)
	if err != nil {
		s.writeRunError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleRunEvents(w http.ResponseWriter, request *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unavailable", "Streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		run, err := s.engine.Get(request.Context(), request.PathValue("id"))
		if err != nil {
			return
		}
		encoded, err := json.Marshal(run)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "event: status\ndata: %s\n\n", encoded)
		flusher.Flush()
		if run.State == domain.RunStateStopped || run.State == domain.RunStateSucceeded || run.State == domain.RunStateFailed {
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) writeRunStartError(w http.ResponseWriter, run engine.RunView, err error) {
	status := http.StatusBadGateway
	code := "worker_start_failed"
	if errors.Is(err, engine.ErrWorkerUnavailable) {
		status = http.StatusServiceUnavailable
		code = "worker_unavailable"
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": "RedisShake worker could not be started",
		},
		"run": run,
	})
}

func (s *Server) writeRunError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Run or task not found")
	case errors.Is(err, store.ErrTaskNotReady):
		writeError(w, http.StatusConflict, "task_not_ready", "Task must be READY at the requested config revision")
	case errors.Is(err, store.ErrActiveRun):
		writeError(w, http.StatusConflict, "active_run_exists", "Task already has an active or unknown run")
	case errors.Is(err, store.ErrConcurrencyLimit):
		writeError(w, http.StatusTooManyRequests, "run_concurrency_limit", "Global active Run limit has been reached")
	case errors.Is(err, store.ErrRevisionConflict):
		writeError(w, http.StatusConflict, "revision_conflict", "Task configuration has changed")
	case errors.Is(err, engine.ErrRunNotManaged):
		writeError(w, http.StatusConflict, "run_not_managed", "Run ownership cannot be proven after control-plane restart")
	case errors.Is(err, engine.ErrWorkerUnavailable):
		writeError(w, http.StatusServiceUnavailable, "worker_unavailable", "RedisShake worker binary is unavailable")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "run_state_conflict", "Run state does not allow this operation")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed")
	}
}

func parseIntegerQuery(request *http.Request, name string, fallback int) (int, error) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

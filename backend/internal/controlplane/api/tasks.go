package api

import (
	"errors"
	"net/http"
	"strconv"

	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/tasks"
)

func (s *Server) handleListTasks(w http.ResponseWriter, request *http.Request) {
	includeArchived := false
	if raw := request.URL.Query().Get("include_archived"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "include_archived must be true or false")
			return
		}
		includeArchived = parsed
	}
	items, err := s.tasks.List(request.Context(), includeArchived)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, request *http.Request) {
	var input tasks.Spec
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	created, err := s.tasks.Create(request.Context(), input)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetTask(w http.ResponseWriter, request *http.Request) {
	task, err := s.tasks.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, request *http.Request) {
	var patch tasks.Patch
	if err := decodeJSON(w, request, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	updated, err := s.tasks.Update(request.Context(), request.PathValue("id"), patch)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleArchiveTask(w http.ResponseWriter, request *http.Request) {
	if err := s.tasks.Archive(request.Context(), request.PathValue("id")); err != nil {
		s.writeTaskError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePrecheckTask(w http.ResponseWriter, request *http.Request) {
	var input tasks.PrecheckRequest
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := s.tasks.Precheck(request.Context(), request.PathValue("id"), input)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCopyTask(w http.ResponseWriter, request *http.Request) {
	var input copyRequest
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	task, err := s.tasks.Copy(request.Context(), request.PathValue("id"), input.Name)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) writeTaskError(w http.ResponseWriter, err error) {
	var validation *tasks.ValidationError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "validation_failed",
				"field":   validation.Field,
				"message": validation.Message,
			},
		})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Task not found")
	case errors.Is(err, store.ErrRevisionConflict):
		writeError(w, http.StatusConflict, "revision_conflict", "Task configuration has changed; reload before retrying")
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "task_in_use", "Task has an active run")
	case errors.Is(err, tasks.ErrArchived):
		writeError(w, http.StatusConflict, "task_archived", "Archived tasks cannot be changed or prechecked")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "task_conflict", "A task with the same name already exists")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed")
	}
}

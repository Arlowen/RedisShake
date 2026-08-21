package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/store"
)

const maxJSONBodySize = 4 << 20

type connectionTestRequest struct {
	Purpose connections.TestPurpose `json:"purpose"`
}

type unsavedConnectionTestRequest struct {
	Connection connections.Spec        `json:"connection"`
	Purpose    connections.TestPurpose `json:"purpose"`
}

func (s *Server) handleListConnections(w http.ResponseWriter, request *http.Request) {
	items, err := s.connections.List(request.Context())
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, request *http.Request) {
	var input connections.Spec
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	created, err := s.connections.Create(request.Context(), input)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetConnection(w http.ResponseWriter, request *http.Request) {
	connection, err := s.connections.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, connection)
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, request *http.Request) {
	var patch connections.Patch
	if err := decodeJSON(w, request, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	updated, err := s.connections.Update(request.Context(), request.PathValue("id"), patch)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, request *http.Request) {
	if err := s.connections.Delete(request.Context(), request.PathValue("id")); err != nil {
		s.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestUnsavedConnection(w http.ResponseWriter, request *http.Request) {
	var input unsavedConnectionTestRequest
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := s.connections.TestUnsaved(request.Context(), input.Connection, input.Purpose)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleTestSavedConnection(w http.ResponseWriter, request *http.Request) {
	var input connectionTestRequest
	if err := decodeJSON(w, request, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := s.connections.TestSaved(request.Context(), request.PathValue("id"), input.Purpose)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	var validation *connections.ValidationError
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
		writeError(w, http.StatusNotFound, "not_found", "Connection not found")
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "connection_in_use", "Connection is referenced by a task")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "connection_conflict", "A connection with the same name already exists")
	case errors.Is(err, connections.ErrSecretsNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "master_key_required", "REDISSHAKE_MASTER_KEY must be configured before saving credentials")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed")
	}
}

func decodeJSON(w http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(w, request.Body, maxJSONBodySize)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON object")
		}
		return err
	}
	return nil
}

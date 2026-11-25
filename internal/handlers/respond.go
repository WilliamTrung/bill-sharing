package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/WilliamTrung/bill-sharing/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if decoder.More() {
		return errors.New("invalid JSON body: unexpected trailing data")
	}
	return nil
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	var id int64
	if _, err := fmt.Sscanf(r.PathValue(name), "%d", &id); err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid identifier")
		return 0, false
	}
	return id, true
}

type badRequestError struct {
	message string
}

func (e *badRequestError) Error() string {
	return e.message
}

func badRequestf(format string, args ...any) error {
	return &badRequestError{message: fmt.Sprintf(format, args...)}
}

func writeServiceError(w http.ResponseWriter, err error) {
	var conflict *service.ConflictError
	if errors.As(err, &conflict) {
		writeError(w, http.StatusBadRequest, conflict.Error())
		return
	}
	if errors.Is(err, service.ErrInvalidTotal) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var badRequest *badRequestError
	if errors.As(err, &badRequest) {
		writeError(w, http.StatusBadRequest, badRequest.Error())
		return
	}
	if strings.Contains(err.Error(), "invalid weight") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "Something went wrong")
}

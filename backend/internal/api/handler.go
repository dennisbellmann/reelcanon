package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/catalog"
)

type Handler struct {
	repository *catalog.Repository
	dbPing     func(context.Context) error
	logger     *slog.Logger
}

func NewHandler(repository *catalog.Repository, dbPing func(context.Context) error, logger *slog.Logger) http.Handler {
	h := &Handler{repository: repository, dbPing: dbPing, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", h.health)
	mux.HandleFunc("GET /api/v1/canons", h.listCanons)
	mux.HandleFunc("GET /api/v1/canons/{slug}", h.getCanon)
	mux.HandleFunc("GET /api/v1/movies/{imdbId}/canons", h.movieCanons)
	return requestLog(logger, mux)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.dbPing(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listCanons(w http.ResponseWriter, r *http.Request) {
	canons, err := h.repository.ListCanons(r.Context())
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, canons)
}

func (h *Handler) getCanon(w http.ResponseWriter, r *http.Request) {
	result, err := h.repository.GetCanon(r.Context(), r.PathValue("slug"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "canon not found"})
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) internalError(w http.ResponseWriter, err error) {
	h.logger.Error("public API request failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func (h *Handler) movieCanons(w http.ResponseWriter, r *http.Request) {
	canons, err := h.repository.MovieCanons(r.Context(), r.PathValue("imdbId"))
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, canons)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

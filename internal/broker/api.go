package broker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/turn-proxy/turn-broker/internal/turn"
)

type server struct {
	cache           *SessionCache
	provider        turn.Provider
	refreshInterval time.Duration
}

func Serve(ctx context.Context, bind string, cache *SessionCache, provider turn.Provider, refreshInterval time.Duration) error {
	s := &server{cache: cache, provider: provider, refreshInterval: refreshInterval}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/session", s.handleSession(ctx))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	slog.Info("broker http listening", "addr", bind)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *server) handleSession(rootCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		joinLink := r.URL.Query().Get("join_link")
		if joinLink == "" {
			http.Error(w, "missing join_link", http.StatusBadRequest)
			return
		}
		session, err := InitializeSession(r.Context(), rootCtx, s.cache, s.provider, joinLink, s.refreshInterval)
		if err != nil {
			slog.Warn("session init failed", "join_link", joinLink, "err", err)
			http.Error(w, "session init failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	}
}

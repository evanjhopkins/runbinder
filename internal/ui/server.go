package ui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/evanjhopkins/RunBinder/internal/app"
)

//go:embed web/*
var webFiles embed.FS

const DefaultAddress = "0.0.0.0:8787"

type Server struct {
	httpServer *http.Server
	listener   net.Listener
}

func Start(ctx context.Context, application *app.Application, address string) (*Server, error) {
	if address == "" {
		address = DefaultAddress
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("start UI listener: %w", err)
	}
	server := &Server{
		listener: listener,
		httpServer: &http.Server{
			Handler: newHandler(application),
		},
	}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	go func() {
		_ = server.httpServer.Serve(listener)
	}()
	return server, nil
}

func (s *Server) URL() string {
	return "http://" + s.listener.Addr().String()
}

func (s *Server) Close() error {
	return s.httpServer.Shutdown(context.Background())
}

func newHandler(application *app.Application) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := application.Service.Status(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		status.RecentLogs, err = application.Service.Logs(500)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, status)
	})
	mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, r *http.Request) {
		tasks, err := application.Tasks.List(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, tasks)
	})
	mux.HandleFunc("GET /api/tasks/{namespace}/runs", func(w http.ResponseWriter, r *http.Request) {
		limit := queryLimit(r, 50, 200)
		runs, err := application.Tasks.Runs(r.Context(), r.PathValue("namespace"), limit)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, runs)
	})
	mux.HandleFunc("GET /api/tasks/{namespace}/log", func(w http.ResponseWriter, r *http.Request) {
		lines := queryLimit(r, 500, 5000)
		log, err := application.Tasks.Log(r.Context(), r.PathValue("namespace"), lines)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, log)
	})
	staticFiles, _ := fs.Sub(webFiles, "web")
	static := http.FileServer(http.FS(staticFiles))
	mux.Handle("/", http.StripPrefix("/", static))
	return loggingHandler(mux)
}

func queryLimit(r *http.Request, fallback, maximum int) int {
	value, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || value < 1 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, err error) {
	http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

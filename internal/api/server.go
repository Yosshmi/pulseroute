package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"pulseroute/internal/config"
	"pulseroute/internal/security"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Config config.Config
	Log    *slog.Logger
}
type contextKey string

const userKey contextKey = "user"
const requestKey contextKey = "request"

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /ready", s.ready)
	m.HandleFunc("POST /api/auth/register", s.register)
	m.HandleFunc("POST /api/auth/login", s.login)
	m.Handle("POST /api/auth/logout", s.auth(http.HandlerFunc(s.logout)))
	m.Handle("GET /api/auth/me", s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"id": user(r)}) })))
	m.HandleFunc("POST /v1/events", s.ingest)
	routes := map[string]http.HandlerFunc{
		"GET /api/projects": s.projects, "POST /api/projects": s.createProject, "GET /api/projects/{id}": s.project,
		"GET /api/projects/{id}/endpoints": s.endpoints, "POST /api/projects/{id}/endpoints": s.createEndpoint,
		"PATCH /api/endpoints/{id}": s.updateEndpoint, "DELETE /api/endpoints/{id}": s.deleteEndpoint,
		"GET /api/projects/{id}/api-keys": s.keys, "POST /api/projects/{id}/api-keys": s.createKey, "DELETE /api/api-keys/{id}": s.deleteKey,
		"GET /api/events": s.events, "GET /api/events/{id}": s.event, "GET /api/deliveries": s.deliveries, "GET /api/deliveries/{id}": s.delivery,
		"POST /api/deliveries/{id}/replay": s.replay, "GET /api/dead-letters": s.deadLetters, "POST /api/dead-letters/{id}/replay": s.replay, "GET /api/dashboard": s.dashboard,
	}
	for route, h := range routes {
		m.Handle(route, s.auth(h))
	}
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, r, 404, "NOT_FOUND", "route not found") })
	m.Handle("GET /", http.FileServer(http.Dir(s.Config.StaticDir)))
	return s.middleware(m)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": r.Context().Value(requestKey)}})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, r, 400, "INVALID_JSON", "invalid JSON body or body exceeds 1 MiB")
		return false
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		fail(w, r, 400, "INVALID_JSON", "exactly one JSON value is required")
		return false
	}
	return true
}
func user(r *http.Request) string { v, _ := r.Context().Value(userKey).(string); return v }
func (s *Server) dbError(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, r, 404, "NOT_FOUND", "resource not found")
		return
	}
	s.Log.Error("database operation failed", "request_id", r.Context().Value(requestKey), "error", e)
	fail(w, r, 500, "INTERNAL", "operation could not be completed")
}
func (s *Server) owner(r *http.Request, id string) bool {
	var ok bool
	e := s.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND owner_id=$2)", id, user(r)).Scan(&ok)
	return e == nil && ok
}
func page(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 25
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	if offset > 100000 {
		offset = 100000
	}
	return limit, offset
}
func (s *Server) rows(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	rows, e := s.DB.Query(r.Context(), query, args...)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	fields := rows.FieldDescriptions()
	for rows.Next() {
		values, e := rows.Values()
		if e != nil {
			s.dbError(w, r, e)
			return
		}
		item := map[string]any{}
		for i, f := range fields {
			item[f.Name] = values[i]
		}
		out = append(out, item)
	}
	if e = rows.Err(); e != nil {
		s.dbError(w, r, e)
		return
	}
	limit, offset := page(r)
	write(w, 200, map[string]any{"items": out, "limit": limit, "offset": offset})
}
func (s *Server) one(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	var b []byte
	e := s.DB.QueryRow(r.Context(), query, args...).Scan(&b)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	write(w, 200, json.RawMessage(b))
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(n int) { w.status = n; w.ResponseWriter.WriteHeader(n) }
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := security.Token("req_")
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), requestKey, id), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		rw := &recorder{w, 200}
		start := time.Now()
		defer func() {
			if recover() != nil {
				s.Log.Error("request panic", "request_id", id)
				fail(rw, r, 500, "INTERNAL", "internal error")
			}
			s.Log.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status_code", rw.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host {
					fail(rw, r, 403, "ORIGIN", "cross-origin request rejected")
					return
				}
			}
			if strings.Contains(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
				fail(rw, r, 403, "ORIGIN", "cross-site request rejected")
				return
			}
		}
		next.ServeHTTP(rw, r)
	})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.DB.Ping(ctx) != nil || s.Redis.Ping(ctx).Err() != nil {
		fail(w, r, 503, "UNAVAILABLE", "dependency unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}

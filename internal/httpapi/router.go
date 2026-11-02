package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/roma007/OpenQuizGo/internal/admin"
	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/question"
	"github.com/roma007/OpenQuizGo/internal/quiz"
)

// Deps 是 HTTP 层依赖。
type Deps struct {
	Logger    *slog.Logger
	DB        *pgxpool.Pool
	Redis     *redis.Client
	Identity  *identity.Service
	Admin     *admin.Service
	Questions *question.Service
	Quiz      *quiz.Service
	Version   string
	Env       string
}

// New 构建路由。
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(RequestID)
	r.Use(Logger(d.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()

		if err := d.DB.Ping(ctx); err != nil {
			WriteError(w, req, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is not ready")
			return
		}
		if d.Redis != nil {
			if err := d.Redis.Ping(ctx).Err(); err != nil {
				WriteError(w, req, http.StatusServiceUnavailable, "CACHE_UNAVAILABLE", "cache is not ready")
				return
			}
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/v1", func(r chi.Router) {
		r.Get("/meta", func(w http.ResponseWriter, _ *http.Request) {
			WriteJSON(w, http.StatusOK, map[string]string{
				"service": "openquizgo",
				"version": d.Version,
				"env":     d.Env,
			})
		})
		if d.Identity != nil {
			d.Identity.Routes(r)
		}
		if d.Admin != nil {
			d.Admin.Routes(r)
		}
		if d.Questions != nil && d.Admin != nil {
			d.Questions.Routes(r, d.Admin)
		}
		if d.Quiz != nil && d.Admin != nil {
			d.Quiz.Routes(r, d.Admin)
		}
	})

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return r
}

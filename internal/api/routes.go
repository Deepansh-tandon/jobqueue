package api

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "jobqueue/internal/middleware"
)

func NewRouter(
    mw *middleware.Middleware,
    registerHandler http.HandlerFunc,
    loginHandler    http.HandlerFunc,
    submitHandler   http.HandlerFunc,
    statusHandler   http.HandlerFunc,
    listHandler     http.HandlerFunc,
) http.Handler {
    r := chi.NewRouter()

    // Public
    r.Post("/api/v1/register", registerHandler)
    r.Post("/api/v1/login",    loginHandler)

    // Metrics
    r.Handle("/metrics", promhttp.Handler())

    // Protected
    r.Group(func(r chi.Router) {
        r.Use(mw.APIKeyAuth, mw.RateLimit)
        r.Post("/api/v1/job/submit", submitHandler)
        r.Get ("/api/v1/job/status/{jobID}", statusHandler)
        r.Get ("/api/v1/job/list",        listHandler) // ?projectID=
    })

    return r
}

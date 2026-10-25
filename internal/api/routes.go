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
	loginHandler http.HandlerFunc,
	createProjectHandler http.HandlerFunc,
	listProjectsHandler http.HandlerFunc,
	submitHandler http.HandlerFunc,
	statusHandler http.HandlerFunc,
	listHandler http.HandlerFunc,
	listFailuresHandler http.HandlerFunc,
	getFailureHandler http.HandlerFunc,
	getFailedJobHandler http.HandlerFunc,
	requeueFailedJobHandler http.HandlerFunc,
	discardFailedJobHandler http.HandlerFunc,
) http.Handler {
	r := chi.NewRouter()

	r.Post("/api/v1/register", registerHandler)
	r.Post("/api/v1/login", loginHandler)
	r.Handle("/metrics", promhttp.Handler())

	r.Group(func(r chi.Router) {
		r.Use(mw.APIKeyAuth, mw.RateLimit)
		r.Post("/api/v1/projects", createProjectHandler)
		r.Get("/api/v1/projects", listProjectsHandler)
		r.Post("/api/v1/job/submit", submitHandler)
		r.Get("/api/v1/job/status/{jobID}", statusHandler)
		r.Get("/api/v1/job/list", listHandler)

		r.Get("/api/v1/failures", listFailuresHandler)
		r.Get("/api/v1/failures/jobs/{jobID}", getFailedJobHandler)
		r.Post("/api/v1/failures/jobs/{jobID}/requeue", requeueFailedJobHandler)
		r.Delete("/api/v1/failures/jobs/{jobID}", discardFailedJobHandler)
		r.Get("/api/v1/failures/{fingerprint}", getFailureHandler)
	})

	return r
}

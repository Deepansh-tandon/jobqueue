package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"jobqueue/internal/heuristics"
	"jobqueue/internal/jobs"
	"jobqueue/internal/middleware"
	"jobqueue/internal/models"
)

type SubmitRequest struct {
	ProjectID string                 `json:"project_id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	JobID     string                 `json:"job_id"`
}

type SubmitResponse struct {
	JobID string `json:"job_id"`
}

func (a *API) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := jobs.ValidateSubmit(req.ProjectID, req.Type, req.Payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, ok := a.loadOwnedProject(w, user, req.ProjectID); !ok {
		return
	}

	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		http.Error(w, "failed to marshal payload", http.StatusBadRequest)
		return
	}

	job := models.Job{
		ID:        uuid.NewString(),
		Type:      req.Type,
		Payload:   string(payloadJSON),
		Status:    models.StatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ProjectID: req.ProjectID,
	}
	if err := a.db.Create(&job).Error; err != nil {
		a.logger.Error("failed to create job", zap.Error(err))
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	queueName := heuristics.GetQueue(req.Type)
	if err := a.rdb.LPush(r.Context(), queueName, job.ID).Err(); err != nil {
		a.logger.Error("failed to enqueue job", zap.Error(err), zap.String("job_id", job.ID))
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(SubmitResponse{JobID: job.ID})
}

func (a *API) StatusHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	jobID := chi.URLParam(r, "jobID")

	var job models.Job
	if err := a.db.First(&job, "id = ?", jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		a.logger.Error("failed to get job", zap.Error(err), zap.String("job_id", jobID))
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}

	if _, ok := a.loadOwnedProject(w, user, job.ProjectID); !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (a *API) ListHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	projectID := r.URL.Query().Get("projectID")
	if projectID == "" {
		http.Error(w, "projectID query parameter is required", http.StatusBadRequest)
		return
	}

	if _, ok := a.loadOwnedProject(w, user, projectID); !ok {
		return
	}

	var jobs []models.Job
	a.db.Where("project_id = ?", projectID).
		Order("created_at desc").
		Find(&jobs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

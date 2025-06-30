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
	_, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Validate that user is a member of req.ProjectID

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

	queueName := heuristics.GetPriorityQueue(req.Type)
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

	// Security Check: Ensure the user has access to the project this job belongs to.
	var project models.Project
	if err := a.db.First(&project, "id = ? AND user_id = ?", job.ProjectID, user.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.logger.Error("failed to get project for auth check", zap.Error(err), zap.String("project_id", job.ProjectID))
		http.Error(w, "internal server error", http.StatusInternalServerError)
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

	// Security Check: Ensure the user has access to the project.
	var project models.Project
	if err := a.db.First(&project, "id = ? AND user_id = ?", projectID, user.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.logger.Error("failed to get project for auth check", zap.Error(err), zap.String("project_id", projectID))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var jobs []models.Job
	a.db.Where("project_id = ?", projectID).
		Order("created_at desc").
		Find(&jobs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

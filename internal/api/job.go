package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	JobID            string `json:"job_id"`
	IdempotentReplay bool   `json:"idempotent_replay"`
}

type StatusResponse struct {
	Job    models.Job        `json:"job"`
	Events []models.JobEvent `json:"events"`
}

func (a *API) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
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

	reqHash, err := jobs.RequestHash(req.ProjectID, req.Type, req.Payload)
	if err != nil {
		http.Error(w, "failed to hash request", http.StatusBadRequest)
		return
	}

	var existing models.Job
	err = a.db.Where("project_id = ? AND idempotency_key = ?", req.ProjectID, key).First(&existing).Error
	if err == nil {
		if existing.RequestHash != reqHash {
			http.Error(w, "idempotency key reused with different body", http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SubmitResponse{JobID: existing.ID, IdempotentReplay: true})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "failed to check idempotency", http.StatusInternalServerError)
		return
	}

	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		http.Error(w, "failed to marshal payload", http.StatusBadRequest)
		return
	}

	jobID := uuid.NewString()
	if req.JobID != "" {
		jobID = req.JobID
	}

	job := models.Job{
		ID:             jobID,
		Type:           req.Type,
		Payload:        string(payloadJSON),
		Status:         models.StatusQueued,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ProjectID:      req.ProjectID,
		IdempotencyKey: key,
		RequestHash:    reqHash,
	}

	tx := a.db.Begin()
	if err := tx.Create(&job).Error; err != nil {
		tx.Rollback()
		if isUniqueViolation(err) {
			var again models.Job
			if a.db.Where("project_id = ? AND idempotency_key = ?", req.ProjectID, key).First(&again).Error == nil {
				if again.RequestHash != reqHash {
					http.Error(w, "idempotency key reused with different body", http.StatusConflict)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(SubmitResponse{JobID: again.ID, IdempotentReplay: true})
				return
			}
		}
		a.logger.Error("failed to create job", zap.Error(err))
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}
	if err := jobs.AppendEvent(tx, job.ID, models.EventQueued, map[string]interface{}{"type": job.Type}); err != nil {
		tx.Rollback()
		http.Error(w, "failed to record event", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit job", http.StatusInternalServerError)
		return
	}

	if err := jobs.EnqueueJobIDCtx(r.Context(), a.rdb, job.Type, job.ID); err != nil {
		a.logger.Error("failed to enqueue job (sweeper will retry)", zap.Error(err), zap.String("job_id", job.ID))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(SubmitResponse{JobID: job.ID, IdempotentReplay: false})
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
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}

	if _, ok := a.loadOwnedProject(w, user, job.ProjectID); !ok {
		return
	}

	var events []models.JobEvent
	a.db.Where("job_id = ?", job.ID).Order("created_at asc").Find(&events)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{Job: job, Events: events})
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

	var list []models.Job
	a.db.Where("project_id = ?", projectID).
		Order("created_at desc").
		Find(&list)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

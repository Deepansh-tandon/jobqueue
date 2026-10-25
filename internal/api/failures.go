package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"jobqueue/internal/jobs"
	"jobqueue/internal/middleware"
	"jobqueue/internal/models"
)

type incidentView struct {
	models.FailureIncident
	RecentJobIDs []string `json:"recent_job_ids"`
}

func (a *API) ListFailuresHandler(w http.ResponseWriter, r *http.Request) {
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

	var fps []string
	a.db.Model(&models.FailureEvent{}).
		Joins("JOIN jobs ON jobs.id = failure_events.job_id").
		Where("jobs.project_id = ?", projectID).
		Distinct("failure_events.fingerprint").
		Pluck("failure_events.fingerprint", &fps)

	var incidents []models.FailureIncident
	if len(fps) > 0 {
		a.db.Where("fingerprint IN ?", fps).Order("last_seen_at desc").Find(&incidents)
	} else {
		incidents = []models.FailureIncident{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(incidents)
}

func (a *API) GetFailureHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	fp := chi.URLParam(r, "fingerprint")
	var inc models.FailureIncident
	if err := a.db.First(&inc, "fingerprint = ?", fp).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var events []models.FailureEvent
	a.db.Where("fingerprint = ?", fp).Order("created_at desc").Limit(50).Find(&events)
	ids := make([]string, 0, len(events))
	for _, e := range events {
		var job models.Job
		if a.db.First(&job, "id = ?", e.JobID).Error != nil {
			continue
		}
		var project models.Project
		if a.db.First(&project, "id = ? AND user_id = ?", job.ProjectID, user.ID).Error != nil {
			continue
		}
		ids = append(ids, e.JobID)
	}
	if len(ids) == 0 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(incidentView{FailureIncident: inc, RecentJobIDs: ids})
}

func (a *API) GetFailedJobHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	jobID := chi.URLParam(r, "jobID")
	var job models.Job
	if err := a.db.First(&job, "id = ?", jobID).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if _, ok := a.loadOwnedProject(w, user, job.ProjectID); !ok {
		return
	}
	var events []models.JobEvent
	a.db.Where("job_id = ?", job.ID).Order("created_at asc").Find(&events)
	var fe models.FailureEvent
	_ = a.db.Where("job_id = ?", job.ID).Order("created_at desc").First(&fe).Error
	var inc models.FailureIncident
	if fe.Fingerprint != "" {
		_ = a.db.First(&inc, "fingerprint = ?", fe.Fingerprint).Error
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job":      job,
		"events":   events,
		"failure":  fe,
		"incident": inc,
	})
}

func (a *API) RequeueFailedJobHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	jobID := chi.URLParam(r, "jobID")
	var job models.Job
	if err := a.db.First(&job, "id = ?", jobID).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if _, ok := a.loadOwnedProject(w, user, job.ProjectID); !ok {
		return
	}
	if job.Status != models.StatusFailed && job.Status != models.StatusDiscarded {
		http.Error(w, "job is not failed", http.StatusBadRequest)
		return
	}

	var body struct {
		Payload map[string]interface{} `json:"payload"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Payload != nil {
		b, err := json.Marshal(body.Payload)
		if err == nil {
			job.Payload = string(b)
		}
	}

	tx := a.db.Begin()
	job.Status = models.StatusQueued
	job.RetryCount = 0
	job.LastError = ""
	job.UpdatedAt = time.Now()
	if err := tx.Save(&job).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to requeue", http.StatusInternalServerError)
		return
	}
	_ = jobs.AppendEvent(tx, job.ID, models.EventRequeued, map[string]interface{}{"by": user.ID})
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to requeue", http.StatusInternalServerError)
		return
	}
	if err := jobs.EnqueueJobIDCtx(r.Context(), a.rdb, job.Type, job.ID); err != nil {
		http.Error(w, "job saved but stream enqueue failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID, "status": job.Status})
}

func (a *API) DiscardFailedJobHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	jobID := chi.URLParam(r, "jobID")
	var job models.Job
	if err := a.db.First(&job, "id = ?", jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	if _, ok := a.loadOwnedProject(w, user, job.ProjectID); !ok {
		return
	}
	a.db.Model(&job).Update("status", models.StatusDiscarded)
	w.WriteHeader(http.StatusNoContent)
}

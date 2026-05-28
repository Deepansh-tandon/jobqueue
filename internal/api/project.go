package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"jobqueue/internal/middleware"
	"jobqueue/internal/models"
)

type CreateProjectRequest struct {
	Name string `json:"name"`
}

func (a *API) CreateProjectHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	project := models.Project{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(req.Name),
		UserID:    user.ID,
		CreatedAt: time.Now(),
	}
	if err := a.db.Create(&project).Error; err != nil {
		a.logger.Error("failed to create project", zap.Error(err))
		http.Error(w, "failed to create project", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(project)
}

func (a *API) ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var projects []models.Project
	if err := a.db.Where("user_id = ?", user.ID).Order("created_at desc").Find(&projects).Error; err != nil {
		a.logger.Error("failed to list projects", zap.Error(err))
		http.Error(w, "failed to list projects", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

// loadOwnedProject returns the project if it belongs to the user, or writes an HTTP error.
func (a *API) loadOwnedProject(w http.ResponseWriter, user models.User, projectID string) (*models.Project, bool) {
	var project models.Project
	err := a.db.First(&project, "id = ? AND user_id = ?", projectID, user.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return nil, false
		}
		a.logger.Error("failed to get project for auth check", zap.Error(err), zap.String("project_id", projectID))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil, false
	}
	return &project, true
}

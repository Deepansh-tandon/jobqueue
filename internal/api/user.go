package api

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/google/uuid"
    "go.uber.org/zap"
    "golang.org/x/crypto/bcrypt"
    "jobqueue/internal/models"
)

type AuthRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    APIKey   string `json:"api_key"`
}

type AuthResponse struct {
    APIKey string `json:"api_key"`
}

func (a *API) RegisterHandler(w http.ResponseWriter, r *http.Request) {
    var req AuthRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        a.logger.Error("failed to hash password", zap.Error(err))
        http.Error(w, "failed to hash password", http.StatusInternalServerError)
        return
    }

    user := models.User{
        ID:        uuid.NewString(),
        Email:     req.Email,
        Password:  string(hash),
        APIKey:    uuid.NewString(),
        CreatedAt: time.Now(),
    }
    if err := a.db.Create(&user).Error; err != nil {
        // This could be a unique constraint violation
        a.logger.Error("failed to create user", zap.Error(err))
        http.Error(w, "failed to create user", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(AuthResponse{APIKey: user.APIKey})
}

func (a *API) LoginHandler(w http.ResponseWriter, r *http.Request) {
    var req AuthRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    var user models.User
    if err := a.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
        http.Error(w, "invalid email or password", http.StatusUnauthorized)
        return
    }

    if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
        http.Error(w, "invalid email or password", http.StatusUnauthorized)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(AuthResponse{APIKey: user.APIKey})
}

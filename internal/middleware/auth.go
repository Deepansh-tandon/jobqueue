package middleware

import (
	"context"
	"net/http"
	"strings"

	"jobqueue/internal/models"

	"gorm.io/gorm"
)

type ctxKey string

const userCtxKey = ctxKey("user")

func (m *Middleware) APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		apiKey := strings.TrimPrefix(auth, "Bearer ")
		var user models.User
		if err := m.DB.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUser(r *http.Request) (models.User, bool) {
	user, ok := r.Context().Value(userCtxKey).(models.User)
	return user, ok
}

package middleware

import (
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
	"golang.org/x/time/rate"
)

type Middleware struct {
	DB    *gorm.DB
	Cache *cache.Cache
}

func (m *Middleware) getLimiter(apiKey string) *rate.Limiter {
	limiter, found := m.Cache.Get(apiKey)
	if !found {
		limiter = rate.NewLimiter(5, 10) // 5 req/sec, burst 10
		m.Cache.Set(apiKey, limiter, 1*time.Hour)
	}
	return limiter.(*rate.Limiter)
}

func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUser(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		limiter := m.getLimiter(user.APIKey)
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

package api

import (
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type API struct {
	db     *gorm.DB
	rdb    *redis.Client
	logger *zap.Logger
}

func New(db *gorm.DB, rdb *redis.Client, logger *zap.Logger) *API {
	return &API{
		db:     db,
		rdb:    rdb,
		logger: logger,
	}
} 
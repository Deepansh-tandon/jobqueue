package workers

import (
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"jobqueue/internal/ai"
	"jobqueue/internal/monitoring"
)

type Pool struct {
	ctx           context.Context
	cancel        context.CancelFunc
	jobQueue      string
	min, max, num int
	nextWorkerID  int
	mu            sync.Mutex
	wg            sync.WaitGroup
	workers       map[int]context.CancelFunc

	// Dependencies
	db      *gorm.DB
	rdb     *redis.Client
	ai      *ai.AI
	metrics *monitoring.Metrics
	logger  *zap.Logger
}

func NewPool(ctx context.Context, queue string, min, max int, db *gorm.DB, rdb *redis.Client, ai *ai.AI, metrics *monitoring.Metrics, logger *zap.Logger) *Pool {
	pCtx, pCancel := context.WithCancel(ctx)
	pool := &Pool{
		ctx:          pCtx,
		cancel:       pCancel,
		jobQueue:     queue,
		min:          min,
		max:          max,
		db:           db,
		rdb:          rdb,
		ai:           ai,
		metrics:      metrics,
		workers:      make(map[int]context.CancelFunc),
		logger:       logger.With(zap.String("queue", queue)),
		nextWorkerID: 1,
	}
	pool.ScaleUp(min) // Start with minimum workers
	return pool
}

func (p *Pool) ScaleUp(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < n && p.num < p.max; i++ {
		workerID := p.nextWorkerID
		p.nextWorkerID++

		wCtx, wCancel := context.WithCancel(p.ctx)
		p.workers[workerID] = wCancel
		p.num++
		p.wg.Add(1)

		worker := NewWorker(workerID, p.jobQueue, p.db, p.rdb, p.ai, p.metrics, p.logger)
		go func(id int) {
			defer func() {
				p.mu.Lock()
				delete(p.workers, id)
				p.mu.Unlock()
				p.wg.Done()
			}()
			worker.Loop(wCtx)
		}(workerID)
	}
	p.metrics.ActiveWorkers.WithLabelValues(p.jobQueue).Set(float64(p.num))
	p.logger.Info("scaled up", zap.Int("total_workers", p.num))
}

func (p *Pool) ScaleDown(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("attempting to scale down", zap.Int("n", n), zap.Int("current_workers", p.num))
	i := 0
	for id, cancel := range p.workers {
		if i >= n || p.num <= p.min {
			break
		}
		cancel()
		delete(p.workers, id)
		p.num--
		i++
	}
	p.metrics.ActiveWorkers.WithLabelValues(p.jobQueue).Set(float64(p.num))
	p.logger.Info("scaled down", zap.Int("total_workers", p.num))
}

func (p *Pool) Shutdown() {
	p.logger.Info("shutting down worker pool")
	p.cancel()
	p.wg.Wait()
	p.logger.Info("worker pool shut down gracefully")
}

func (p *Pool) GetStats() (numWorkers int, queueName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.num, p.jobQueue
}

package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"lysis/internal/config"
)

type Job struct {
	ScanID string
	UserID int
	Run    func(ctx context.Context) error
}

type Pool struct {
	jobs   chan Job
	cfg    config.LimitsConfig
	active map[string]int
	mu     sync.Mutex
}

func NewPool(cfg config.LimitsConfig) *Pool {
	return &Pool{
		jobs:   make(chan Job, 1000),
		cfg:    cfg,
		active: make(map[string]int),
	}
}

func (p *Pool) Submit(job Job) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

func (p *Pool) Start(ctx context.Context, workerCount int) {
	for i := 0; i < workerCount; i++ {
		go p.worker(ctx, i)
	}
}

func (p *Pool) worker(ctx context.Context, id int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[pool] worker %d recovered from panic: %v", id, r)
			go p.worker(ctx, id)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.mu.Lock()
			p.active[job.ScanID] = job.UserID
			p.mu.Unlock()

			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[pool] job %s panicked: %v", job.ScanID, r)
					}
				}()

				timeoutStr := p.cfg.ScanTimeout
				d, err := time.ParseDuration(timeoutStr)
				if err != nil || d <= 0 {
					d = 30 * time.Minute
				}
				jobCtx, cancel := context.WithTimeout(ctx, d)
				defer cancel()

				job.Run(jobCtx)
			}()

			p.mu.Lock()
			delete(p.active, job.ScanID)
			p.mu.Unlock()
		}
	}
}

func (p *Pool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.active)
}

func (p *Pool) WaitDrain(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("drain timeout: %d jobs still active", p.ActiveCount())
		case <-ticker.C:
			if p.ActiveCount() == 0 {
				return nil
			}
		}
	}
}

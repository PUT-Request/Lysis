package middleware

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"lysis/internal/config"
)

type RateLimiter struct {
	mu              sync.Mutex
	userDailyCount  map[int]*dailyCounter
	userActiveCount map[int]int32
	ipMinuteCount   map[string]*minuteCounter
	globalActive    int32
	limits          config.LimitsConfig
	done            chan struct{}
}

type dailyCounter struct {
	count  int32
	resetAt time.Time
}

type minuteCounter struct {
	count   int32
	resetAt time.Time
}

func NewRateLimiter(cfg config.LimitsConfig) *RateLimiter {
	rl := &RateLimiter{
		userDailyCount:  make(map[int]*dailyCounter),
		userActiveCount: make(map[int]int32),
		ipMinuteCount:   make(map[string]*minuteCounter),
		limits:          cfg,
		done:            make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for uid, c := range rl.userDailyCount {
				if now.After(c.resetAt) {
					delete(rl.userDailyCount, uid)
				}
			}
			for ip, c := range rl.ipMinuteCount {
				if now.After(c.resetAt) {
					delete(rl.ipMinuteCount, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.done)
}

func (rl *RateLimiter) AllowNewScan(userID int) (bool, string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	c, ok := rl.userDailyCount[userID]
	if !ok || now.After(c.resetAt) {
		c = &dailyCounter{resetAt: now.Add(24 * time.Hour)}
		rl.userDailyCount[userID] = c
	}
	if int(c.count) >= rl.limits.ScansPerUserPerDay {
		return false, "daily scan limit reached (100), try again tomorrow"
	}

	active := rl.userActiveCount[userID]
	if int(active) >= rl.limits.MaxConcurrentPerUser {
		return false, "concurrent scan limit reached (2), wait for one to finish"
	}

	if int(atomic.LoadInt32(&rl.globalActive)) >= rl.limits.MaxGlobalConcurrent {
		return false, "too many scans in progress, try again later"
	}

	c.count++
	rl.userActiveCount[userID]++
	atomic.AddInt32(&rl.globalActive, 1)
	return true, ""
}

func (rl *RateLimiter) AllowFromIP(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	c, ok := rl.ipMinuteCount[ip]
	if !ok || now.After(c.resetAt) {
		c = &minuteCounter{resetAt: now.Add(1 * time.Minute)}
		rl.ipMinuteCount[ip] = c
	}

	maxPerMinute := 20
	if rl.limits.ScansPerIPPerDay > 0 {
		maxPerMinute = (rl.limits.ScansPerIPPerDay + 1439) / 1440
	}

	if int(c.count) >= maxPerMinute {
		return false
	}
	c.count++
	return true
}

func (rl *RateLimiter) ReleaseScan(userID int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if v, ok := rl.userActiveCount[userID]; ok && v > 0 {
		rl.userActiveCount[userID]--
	}
	if atomic.LoadInt32(&rl.globalActive) > 0 {
		atomic.AddInt32(&rl.globalActive, -1)
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/api/scans") {
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = fwd
			}
			if !rl.AllowFromIP(ip) {
				http.Error(w, `{"error":"rate limit exceeded from this IP"}`, http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func DiskCheckMiddleware(minFreeBytes int64, tempDir string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/scans") && (r.Method == http.MethodPost) {
				var stat syscall.Statfs_t
				if err := syscall.Statfs(tempDir, &stat); err == nil {
					free := int64(stat.Bavail) * int64(stat.Bsize)
					if free < minFreeBytes {
						http.Error(w, `{"error":"disk space low, try again later"}`, http.StatusServiceUnavailable)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

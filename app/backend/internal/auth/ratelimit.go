package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter tracks and enforces request rate limits per client IP.
type RateLimiter struct {
	mu          sync.Mutex
	limit       int           // max requests allowed per window
	window      time.Duration // sliding window duration
	visitors    map[string]*visitor
	cleanupStop chan struct{}
}

type visitor struct {
	requests []time.Time
}

// NewRateLimiter creates a new RateLimiter.
// limit: number of requests allowed within the window.
// window: duration of the window (e.g. 1 * time.Minute).
// cleanupInterval: how often to clean up stale visitor records (if <= 0, defaults to 2 * window).
func NewRateLimiter(limit int, window time.Duration, cleanupInterval ...time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}

	interval := 2 * window
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		interval = cleanupInterval[0]
	}

	rl := &RateLimiter{
		limit:       limit,
		window:      window,
		visitors:    make(map[string]*visitor),
		cleanupStop: make(chan struct{}),
	}

	go rl.cleanupLoop(interval)

	return rl
}

// Close stops the background cleanup goroutine.
func (rl *RateLimiter) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	select {
	case <-rl.cleanupStop:
		// already closed
	default:
		close(rl.cleanupStop)
	}
}

func (rl *RateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.Cleanup(time.Now())
		case <-rl.cleanupStop:
			return
		}
	}
}

// Cleanup removes entries with no active requests within the window.
func (rl *RateLimiter) Cleanup(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)
	for ip, v := range rl.visitors {
		validIdx := 0
		for _, reqTime := range v.requests {
			if reqTime.After(cutoff) {
				v.requests[validIdx] = reqTime
				validIdx++
			}
		}
		v.requests = v.requests[:validIdx]
		if len(v.requests) == 0 {
			delete(rl.visitors, ip)
		}
	}
}

// Allow checks whether a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string, now time.Time) (bool, int, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{requests: make([]time.Time, 0, rl.limit)}
		rl.visitors[ip] = v
	}

	validIdx := 0
	for _, reqTime := range v.requests {
		if reqTime.After(cutoff) {
			v.requests[validIdx] = reqTime
			validIdx++
		}
	}
	v.requests = v.requests[:validIdx]

	if len(v.requests) >= rl.limit {
		oldest := v.requests[0]
		resetDuration := oldest.Add(rl.window).Sub(now)
		if resetDuration < 0 {
			resetDuration = time.Second
		}
		return false, 0, resetDuration
	}

	v.requests = append(v.requests, now)
	remaining := rl.limit - len(v.requests)
	resetDuration := rl.window
	return true, remaining, resetDuration
}

// ExtractIP extracts the client IP address from request headers or remote address.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// Middleware returns an HTTP middleware enforcing rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now().UTC()
		ip := ExtractIP(r)
		allowed, remaining, resetDuration := rl.Allow(ip, now)

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(resetDuration).Unix(), 10))

		if !allowed {
			retrySeconds := int(resetDuration.Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Too many requests, please try again later",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

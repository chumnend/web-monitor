package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"web-monitor/internal/auth"
)

func TestRateLimiterAllow(t *testing.T) {
	window := 200 * time.Millisecond
	limit := 3
	rl := auth.NewRateLimiter(limit, window, 100*time.Millisecond)
	defer rl.Close()

	ip := "192.168.1.50"
	now := time.Now().UTC()

	// 1st request
	allowed, remaining, _ := rl.Allow(ip, now)
	if !allowed || remaining != 2 {
		t.Errorf("req 1: expected allowed true, remaining 2; got %v, %d", allowed, remaining)
	}

	// 2nd request
	allowed, remaining, _ = rl.Allow(ip, now.Add(10*time.Millisecond))
	if !allowed || remaining != 1 {
		t.Errorf("req 2: expected allowed true, remaining 1; got %v, %d", allowed, remaining)
	}

	// 3rd request
	allowed, remaining, _ = rl.Allow(ip, now.Add(20*time.Millisecond))
	if !allowed || remaining != 0 {
		t.Errorf("req 3: expected allowed true, remaining 0; got %v, %d", allowed, remaining)
	}

	// 4th request (should be rejected)
	allowed, remaining, resetDuration := rl.Allow(ip, now.Add(30*time.Millisecond))
	if allowed {
		t.Errorf("req 4: expected allowed false; got true")
	}
	if remaining != 0 {
		t.Errorf("req 4: expected remaining 0; got %d", remaining)
	}
	if resetDuration <= 0 {
		t.Errorf("expected positive resetDuration, got %v", resetDuration)
	}

	// Different IP should still be allowed
	diffIP := "10.0.0.1"
	allowedDiff, remainingDiff, _ := rl.Allow(diffIP, now.Add(30*time.Millisecond))
	if !allowedDiff || remainingDiff != 2 {
		t.Errorf("diff IP: expected allowed true, remaining 2; got %v, %d", allowedDiff, remainingDiff)
	}

	// After window expires for first IP
	allowedAfter, _, _ := rl.Allow(ip, now.Add(window+10*time.Millisecond))
	if !allowedAfter {
		t.Errorf("expected allowed true after window expired")
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	window := 50 * time.Millisecond
	rl := auth.NewRateLimiter(2, window, 20*time.Millisecond)
	defer rl.Close()

	now := time.Now().UTC()
	rl.Allow("1.1.1.1", now)

	// Clean up after window expires
	rl.Cleanup(now.Add(window + 10*time.Millisecond))

	// Should allow fresh limit count
	allowed, remaining, _ := rl.Allow("1.1.1.1", now.Add(window+20*time.Millisecond))
	if !allowed || remaining != 1 {
		t.Errorf("expected fresh remaining 1, got allowed %v, remaining %d", allowed, remaining)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		xri      string
		remote   string
		expected string
	}{
		{
			name:     "X-Forwarded-For single",
			xff:      "203.0.113.195",
			expected: "203.0.113.195",
		},
		{
			name:     "X-Forwarded-For multiple",
			xff:      "203.0.113.195, 70.41.3.18, 150.172.238.178",
			expected: "203.0.113.195",
		},
		{
			name:     "X-Real-IP fallback",
			xri:      "198.51.100.1",
			expected: "198.51.100.1",
		},
		{
			name:     "RemoteAddr with port",
			remote:   "192.0.2.1:54321",
			expected: "192.0.2.1",
		},
		{
			name:     "RemoteAddr without port",
			remote:   "192.0.2.2",
			expected: "192.0.2.2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xri != "" {
				req.Header.Set("X-Real-IP", tc.xri)
			}
			if tc.remote != "" {
				req.RemoteAddr = tc.remote
			}

			ip := auth.ExtractIP(req)
			if ip != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, ip)
			}
		})
	}
}

func TestRateLimiterMiddleware(t *testing.T) {
	limit := 2
	window := 1 * time.Minute
	rl := auth.NewRateLimiter(limit, window)
	defer rl.Close()

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := rl.Middleware(dummyHandler)

	// 1st request -> OK
	req1 := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("req 1: expected 200, got %d", rec1.Code)
	}
	if rec1.Header().Get("X-RateLimit-Limit") != "2" {
		t.Errorf("expected X-RateLimit-Limit 2, got %s", rec1.Header().Get("X-RateLimit-Limit"))
	}
	if rec1.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Errorf("expected X-RateLimit-Remaining 1, got %s", rec1.Header().Get("X-RateLimit-Remaining"))
	}

	// 2nd request -> OK
	req2 := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req2.RemoteAddr = "1.2.3.4:1234"
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("req 2: expected 200, got %d", rec2.Code)
	}
	if rec2.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining 0, got %s", rec2.Header().Get("X-RateLimit-Remaining"))
	}

	// 3rd request -> 429 Too Many Requests
	req3 := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req3.RemoteAddr = "1.2.3.4:1234"
	rec3 := httptest.NewRecorder()
	mw.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("req 3: expected 429, got %d", rec3.Code)
	}
	if rec3.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set")
	}

	var errBody map[string]string
	if err := json.NewDecoder(rec3.Body).Decode(&errBody); err != nil {
		t.Fatalf("failed to decode 429 body: %v", err)
	}
	if errBody["error"] != "Too many requests, please try again later" {
		t.Errorf("unexpected error message: %s", errBody["error"])
	}

	// OPTIONS preflight should bypass rate limit
	reqOpt := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	reqOpt.RemoteAddr = "1.2.3.4:1234"
	recOpt := httptest.NewRecorder()
	mw.ServeHTTP(recOpt, reqOpt)
	if recOpt.Code != http.StatusOK {
		t.Errorf("OPTIONS request: expected 200, got %d", recOpt.Code)
	}
}

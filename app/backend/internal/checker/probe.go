package checker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"web-monitor/internal/store"
)

// ProbeResult holds the full diagnostic outcome of an individual HTTP probe.
type ProbeResult struct {
	StatusCode      *int
	ResponseTimeMs  *int
	IsUp            bool
	ErrorType       *string
	ErrorMessage    *string
	TimingBreakdown *store.TimingBreakdown
}

// ProbeOptions configures an HTTP probe execution.
type ProbeOptions struct {
	TimeoutSec        int
	ExpectedStatusMin int
	ExpectedStatusMax int
	Method            string // defaults to GET
}

// Probe performs a single HTTP probe against a target URL, gathering latency breakdown metrics.
func Probe(ctx context.Context, client *http.Client, targetURL string, opts ProbeOptions) *ProbeResult {
	if client == nil {
		client = http.DefaultClient
	}

	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}

	timeout := time.Duration(opts.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	minStatus := opts.ExpectedStatusMin
	if minStatus <= 0 {
		minStatus = 200
	}
	maxStatus := opts.ExpectedStatusMax
	if maxStatus <= 0 {
		maxStatus = 399
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var dnsStart, dnsDone time.Time
	var connStart, connDone time.Time
	var tlsStart, tlsDone time.Time
	var gotFirstByte time.Time

	var dnsMs, tcpMs, tlsMs, firstByteMs *int

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
			ms := int(dnsDone.Sub(dnsStart).Milliseconds())
			dnsMs = &ms
		},
		ConnectStart: func(_, _ string) {
			connStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			connDone = time.Now()
			ms := int(connDone.Sub(connStart).Milliseconds())
			tcpMs = &ms
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			tlsDone = time.Now()
			ms := int(tlsDone.Sub(tlsStart).Milliseconds())
			tlsMs = &ms
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(reqCtx, trace), method, targetURL, nil)
	if err != nil {
		errType := "url_parse"
		errMsg := err.Error()
		return &ProbeResult{
			IsUp:         false,
			ErrorType:    &errType,
			ErrorMessage: &errMsg,
		}
	}
	req.Header.Set("User-Agent", "WebMonitor-Probe/1.0")

	startTime := time.Now()
	resp, err := client.Do(req)
	totalMs := int(time.Since(startTime).Milliseconds())

	if !gotFirstByte.IsZero() {
		fb := int(gotFirstByte.Sub(startTime).Milliseconds())
		firstByteMs = &fb
	}

	breakdown := &store.TimingBreakdown{
		DNSLookupMs:    dnsMs,
		TCPConnectMs:   tcpMs,
		TLSHandshakeMs: tlsMs,
		FirstByteMs:    firstByteMs,
		TotalMs:        &totalMs,
	}

	if err != nil {
		errType, errMsg := classifyError(err, reqCtx)
		return &ProbeResult{
			ResponseTimeMs:  &totalMs,
			IsUp:            false,
			ErrorType:       &errType,
			ErrorMessage:    &errMsg,
			TimingBreakdown: breakdown,
		}
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)

	statusCode := resp.StatusCode
	isUp := statusCode >= minStatus && statusCode <= maxStatus

	var errType *string
	var errMsg *string
	if !isUp {
		et := "http_status"
		em := fmt.Sprintf("HTTP status %d outside expected range [%d, %d]", statusCode, minStatus, maxStatus)
		errType = &et
		errMsg = &em
	}

	return &ProbeResult{
		StatusCode:      &statusCode,
		ResponseTimeMs:  &totalMs,
		IsUp:            isUp,
		ErrorType:       errType,
		ErrorMessage:    errMsg,
		TimingBreakdown: breakdown,
	}
}

// ProbeWithRetry executes Probe, retrying up to site.RetryCount on failure.
func ProbeWithRetry(ctx context.Context, client *http.Client, site *store.Site, retryDelay ...time.Duration) *ProbeResult {
	opts := ProbeOptions{
		TimeoutSec:        site.TimeoutSec,
		ExpectedStatusMin: site.ExpectedStatusMin,
		ExpectedStatusMax: site.ExpectedStatusMax,
	}

	delay := 200 * time.Millisecond
	if len(retryDelay) > 0 && retryDelay[0] >= 0 {
		delay = retryDelay[0]
	}

	maxRetries := site.RetryCount
	if maxRetries < 0 {
		maxRetries = 0
	}

	result := Probe(ctx, client, site.URL, opts)
	if result.IsUp || maxRetries == 0 {
		return result
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return result
		case <-time.After(delay):
		}

		retryRes := Probe(ctx, client, site.URL, opts)
		if retryRes.IsUp {
			return retryRes
		}
		result = retryRes
	}

	return result
}

func classifyError(err error, ctx context.Context) (string, string) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return store.ErrorTypeTimeout, "request timed out"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return store.ErrorTypeTimeout, "connection timed out"
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return store.ErrorTypeDNS, fmt.Sprintf("DNS lookup failed: %v", dnsErr)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if strings.Contains(opErr.Error(), "connection refused") {
			return store.ErrorTypeConnectionRefused, "connection refused"
		}
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "tls") || strings.Contains(errStr, "certificate") || strings.Contains(errStr, "handshake") {
		return store.ErrorTypeTLS, fmt.Sprintf("TLS handshake failed: %v", err)
	}

	if strings.Contains(errStr, "connection refused") {
		return store.ErrorTypeConnectionRefused, "connection refused"
	}

	return "network_error", err.Error()
}

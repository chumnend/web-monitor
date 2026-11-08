package sites

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL = errors.New("invalid URL: must be a valid http or https URL with a hostname")
)

// ValidateAndNormalizeURL validates an incoming URL string and generates:
// - displayURL: the formatted display and probe URL with a scheme (defaults to https://)
// - normalizedURL: a canonical representation (host + path + query) used for deduplication
func ValidateAndNormalizeURL(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ErrInvalidURL
	}

	// Default to https if scheme is missing
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", ErrInvalidURL
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", ErrInvalidURL
	}

	host := strings.ToLower(parsed.Host)
	if host == "" {
		return "", "", ErrInvalidURL
	}

	// Extract hostname without port for validation
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	if hostname == "" || strings.ContainsAny(hostname, " /\\?#@:") {
		return "", "", ErrInvalidURL
	}

	// Must be localhost, a valid IP, or contain a domain dot
	if hostname != "localhost" && !strings.Contains(hostname, ".") && net.ParseIP(hostname) == nil {
		return "", "", ErrInvalidURL
	}

	// Remove default ports
	if scheme == "http" && strings.HasSuffix(host, ":80") {
		host = strings.TrimSuffix(host, ":80")
	} else if scheme == "https" && strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}

	// Normalize path (strip trailing slash if root path)
	path := parsed.Path
	if path == "/" {
		path = ""
	}

	// Reconstruct display URL (strip fragment for probing and display)
	displayURL := scheme + "://" + host + path
	if parsed.RawQuery != "" {
		displayURL += "?" + parsed.RawQuery
	}

	// Normalized URL: host + path (+ sorted query without fragment)
	normalizedURL := host + path
	if parsed.RawQuery != "" {
		q := parsed.Query()
		normalizedURL += "?" + q.Encode()
	}

	return displayURL, normalizedURL, nil
}

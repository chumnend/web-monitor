package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PostgresStore implements Store using a PostgreSQL database.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore instance.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// DB returns the underlying *sql.DB connection pool.
func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

// Ping verifies that the database connection is alive.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// CreateUser inserts a new user record into the users table.
func (s *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	if user.Timezone == "" {
		user.Timezone = "UTC"
	}
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	query := `
		INSERT INTO users (id, email, password_hash, timezone, email_verified_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Timezone,
		user.EmailVerifiedAt,
		user.CreatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	return nil
}

// GetUserByEmail retrieves a user by email address (case-insensitive).
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	query := `
		SELECT id, email, password_hash, timezone, email_verified_at, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var user User
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Timezone,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by their UUID.
func (s *PostgresStore) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, email, password_hash, timezone, email_verified_at, created_at
		FROM users
		WHERE id = $1
	`

	var user User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Timezone,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// CreateRefreshToken inserts a new refresh token into the refresh_tokens table.
func (s *PostgresStore) CreateRefreshToken(ctx context.Context, token *RefreshToken) error {
	if token == nil {
		return errors.New("refresh token cannot be nil")
	}
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.db.ExecContext(
		ctx,
		query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.RevokedAt,
		token.CreatedAt,
	)
	return err
}

// GetRefreshToken retrieves a refresh token by its hash.
func (s *PostgresStore) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var token RefreshToken
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// RevokeRefreshToken marks a specific refresh token as revoked.
func (s *PostgresStore) RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE token_hash = $2 AND revoked_at IS NULL
	`
	res, err := s.db.ExecContext(ctx, query, revokedAt, tokenHash)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var count int
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE token_hash = $1`, tokenHash).Scan(&count)
		if count == 0 {
			return ErrRefreshTokenNotFound
		}
	}
	return nil
}

// RevokeUserRefreshTokens revokes all active refresh tokens belonging to a user.
func (s *PostgresStore) RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE user_id = $2 AND revoked_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, revokedAt, userID)
	return err
}

// CreateSite creates a new site, its initial state, and attaches tags within a transaction.
func (s *PostgresStore) CreateSite(ctx context.Context, site *Site, tags []string) (*SiteWithTags, error) {
	if site == nil {
		return nil, errors.New("site cannot be nil")
	}
	if site.UserID == uuid.Nil {
		return nil, errors.New("site must have a valid user_id")
	}

	if site.ID == uuid.Nil {
		site.ID = uuid.New()
	}
	if site.CreatedAt.IsZero() {
		site.CreatedAt = time.Now().UTC()
	}
	if site.CheckIntervalSec <= 0 {
		site.CheckIntervalSec = 60
	}
	if site.TimeoutSec <= 0 {
		site.TimeoutSec = 10
	}
	if site.FailureThreshold <= 0 {
		site.FailureThreshold = 2
	}
	if site.RetryCount < 0 {
		site.RetryCount = 1
	}
	if site.ExpectedStatusMin <= 0 {
		site.ExpectedStatusMin = 200
	}
	if site.ExpectedStatusMax <= 0 {
		site.ExpectedStatusMax = 399
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert into sites
	siteQuery := `
		INSERT INTO sites (
			id, user_id, url, normalized_url, label,
			check_interval_sec, timeout_sec, failure_threshold, retry_count,
			expected_status_min, expected_status_max, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = tx.ExecContext(
		ctx, siteQuery,
		site.ID, site.UserID, site.URL, site.NormalizedURL, site.Label,
		site.CheckIntervalSec, site.TimeoutSec, site.FailureThreshold, site.RetryCount,
		site.ExpectedStatusMin, site.ExpectedStatusMax, site.CreatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrDuplicateSite
		}
		return nil, err
	}

	// Insert initial site_state
	stateQuery := `
		INSERT INTO site_state (site_id, current_status, consecutive_failures)
		VALUES ($1, $2, $3)
	`
	_, err = tx.ExecContext(ctx, stateQuery, site.ID, string(SiteStatusPending), 0)
	if err != nil {
		return nil, err
	}

	// Insert tags and site_tags
	normalizedTags, err := s.attachTagsTx(ctx, tx, site.UserID, site.ID, tags)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &SiteWithTags{
		Site:          *site,
		Tags:          normalizedTags,
		CurrentStatus: SiteStatusPending,
	}, nil
}

// GetSiteByID retrieves an active site by ID for the specified user.
func (s *PostgresStore) GetSiteByID(ctx context.Context, userID, siteID uuid.UUID) (*SiteWithTags, error) {
	query := `
		SELECT s.id, s.user_id, s.url, s.normalized_url, s.label,
		       s.check_interval_sec, s.timeout_sec, s.failure_threshold, s.retry_count,
		       s.expected_status_min, s.expected_status_max, s.created_at, s.deleted_at,
		       COALESCE(st.current_status, 'pending')
		FROM sites s
		LEFT JOIN site_state st ON st.site_id = s.id
		WHERE s.id = $1 AND s.user_id = $2 AND s.deleted_at IS NULL
	`

	var site SiteWithTags
	var statusStr string
	err := s.db.QueryRowContext(ctx, query, siteID, userID).Scan(
		&site.ID, &site.UserID, &site.URL, &site.NormalizedURL, &site.Label,
		&site.CheckIntervalSec, &site.TimeoutSec, &site.FailureThreshold, &site.RetryCount,
		&site.ExpectedStatusMin, &site.ExpectedStatusMax, &site.CreatedAt, &site.DeletedAt,
		&statusStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}
	site.CurrentStatus = SiteStatus(statusStr)

	// Fetch tags
	tags, err := s.getSiteTags(ctx, siteID)
	if err != nil {
		return nil, err
	}
	site.Tags = tags

	return &site, nil
}

// ListSites lists all active sites for a user, optionally filtered by tag.
func (s *PostgresStore) ListSites(ctx context.Context, userID uuid.UUID, tagFilter string) ([]*SiteWithTags, error) {
	tagFilter = strings.TrimSpace(tagFilter)

	var query string
	var args []any

	if tagFilter != "" {
		query = `
			SELECT s.id, s.user_id, s.url, s.normalized_url, s.label,
			       s.check_interval_sec, s.timeout_sec, s.failure_threshold, s.retry_count,
			       s.expected_status_min, s.expected_status_max, s.created_at, s.deleted_at,
			       COALESCE(st.current_status, 'pending')
			FROM sites s
			LEFT JOIN site_state st ON st.site_id = s.id
			JOIN site_tags stg ON stg.site_id = s.id
			JOIN tags t ON t.id = stg.tag_id
			WHERE s.user_id = $1 AND s.deleted_at IS NULL AND LOWER(t.name) = LOWER($2)
			ORDER BY s.created_at DESC
		`
		args = []any{userID, tagFilter}
	} else {
		query = `
			SELECT s.id, s.user_id, s.url, s.normalized_url, s.label,
			       s.check_interval_sec, s.timeout_sec, s.failure_threshold, s.retry_count,
			       s.expected_status_min, s.expected_status_max, s.created_at, s.deleted_at,
			       COALESCE(st.current_status, 'pending')
			FROM sites s
			LEFT JOIN site_state st ON st.site_id = s.id
			WHERE s.user_id = $1 AND s.deleted_at IS NULL
			ORDER BY s.created_at DESC
		`
		args = []any{userID}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []*SiteWithTags
	for rows.Next() {
		var site SiteWithTags
		var statusStr string
		if err := rows.Scan(
			&site.ID, &site.UserID, &site.URL, &site.NormalizedURL, &site.Label,
			&site.CheckIntervalSec, &site.TimeoutSec, &site.FailureThreshold, &site.RetryCount,
			&site.ExpectedStatusMin, &site.ExpectedStatusMax, &site.CreatedAt, &site.DeletedAt,
			&statusStr,
		); err != nil {
			return nil, err
		}
		site.CurrentStatus = SiteStatus(statusStr)
		site.Tags = []string{}
		sites = append(sites, &site)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch tags for each site
	for _, site := range sites {
		tags, err := s.getSiteTags(ctx, site.ID)
		if err != nil {
			return nil, err
		}
		site.Tags = tags
	}

	if sites == nil {
		sites = []*SiteWithTags{}
	}
	return sites, nil
}

// UpdateSite updates specified properties of a site within a transaction.
func (s *PostgresStore) UpdateSite(ctx context.Context, userID, siteID uuid.UUID, input UpdateSiteInput) (*SiteWithTags, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check existence and lock row
	var site Site
	checkQuery := `
		SELECT id, user_id, url, normalized_url, label,
		       check_interval_sec, timeout_sec, failure_threshold, retry_count,
		       expected_status_min, expected_status_max, created_at, deleted_at
		FROM sites
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`
	err = tx.QueryRowContext(ctx, checkQuery, siteID, userID).Scan(
		&site.ID, &site.UserID, &site.URL, &site.NormalizedURL, &site.Label,
		&site.CheckIntervalSec, &site.TimeoutSec, &site.FailureThreshold, &site.RetryCount,
		&site.ExpectedStatusMin, &site.ExpectedStatusMax, &site.CreatedAt, &site.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSiteNotFound
		}
		return nil, err
	}

	// Apply updates
	if input.Label != nil {
		site.Label = input.Label
	}
	if input.CheckIntervalSec != nil {
		site.CheckIntervalSec = *input.CheckIntervalSec
	}
	if input.TimeoutSec != nil {
		site.TimeoutSec = *input.TimeoutSec
	}
	if input.FailureThreshold != nil {
		site.FailureThreshold = *input.FailureThreshold
	}
	if input.RetryCount != nil {
		site.RetryCount = *input.RetryCount
	}
	if input.ExpectedStatusMin != nil {
		site.ExpectedStatusMin = *input.ExpectedStatusMin
	}
	if input.ExpectedStatusMax != nil {
		site.ExpectedStatusMax = *input.ExpectedStatusMax
	}

	updateQuery := `
		UPDATE sites
		SET label = $1, check_interval_sec = $2, timeout_sec = $3,
		    failure_threshold = $4, retry_count = $5,
		    expected_status_min = $6, expected_status_max = $7
		WHERE id = $8
	`
	_, err = tx.ExecContext(
		ctx, updateQuery,
		site.Label, site.CheckIntervalSec, site.TimeoutSec,
		site.FailureThreshold, site.RetryCount,
		site.ExpectedStatusMin, site.ExpectedStatusMax,
		site.ID,
	)
	if err != nil {
		return nil, err
	}

	if input.Tags != nil {
		// Clear old site tags
		if _, err := tx.ExecContext(ctx, `DELETE FROM site_tags WHERE site_id = $1`, site.ID); err != nil {
			return nil, err
		}
		if _, err := s.attachTagsTx(ctx, tx, userID, site.ID, *input.Tags); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetSiteByID(ctx, userID, siteID)
}

// DeleteSite soft-deletes a site by setting deleted_at to NOW().
func (s *PostgresStore) DeleteSite(ctx context.Context, userID, siteID uuid.UUID) error {
	query := `
		UPDATE sites
		SET deleted_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`
	res, err := s.db.ExecContext(ctx, query, siteID, userID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSiteNotFound
	}

	return nil
}

// GetAllActiveSites returns all active (non-deleted) sites across all users.
func (s *PostgresStore) GetAllActiveSites(ctx context.Context) ([]*Site, error) {
	query := `
		SELECT id, user_id, url, normalized_url, label,
		       check_interval_sec, timeout_sec, failure_threshold, retry_count,
		       expected_status_min, expected_status_max, created_at, deleted_at
		FROM sites
		WHERE deleted_at IS NULL
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []*Site
	for rows.Next() {
		var site Site
		if err := rows.Scan(
			&site.ID, &site.UserID, &site.URL, &site.NormalizedURL, &site.Label,
			&site.CheckIntervalSec, &site.TimeoutSec, &site.FailureThreshold, &site.RetryCount,
			&site.ExpectedStatusMin, &site.ExpectedStatusMax, &site.CreatedAt, &site.DeletedAt,
		); err != nil {
			return nil, err
		}
		sites = append(sites, &site)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sites == nil {
		sites = []*Site{}
	}
	return sites, nil
}

// RecordCheckResult inserts a new check result into the check_results table.
func (s *PostgresStore) RecordCheckResult(ctx context.Context, result *CheckResult) error {
	if result == nil {
		return errors.New("check result cannot be nil")
	}

	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now().UTC()
	}

	var timingJSON any
	if len(result.TimingBreakdown) > 0 {
		timingJSON = string(result.TimingBreakdown)
	}

	query := `
		INSERT INTO check_results (
			site_id, checked_at, status_code, response_time_ms,
			is_up, error_type, error_message, timing_breakdown
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err := s.db.QueryRowContext(
		ctx, query,
		result.SiteID, result.CheckedAt, result.StatusCode, result.ResponseTimeMs,
		result.IsUp, result.ErrorType, result.ErrorMessage, timingJSON,
	).Scan(&result.ID)
	return err
}

// GetCheckResults fetches recent check results for a site ordered newest first.
func (s *PostgresStore) GetCheckResults(ctx context.Context, siteID uuid.UUID, limit, offset int) ([]*CheckResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, site_id, checked_at, status_code, response_time_ms,
		       is_up, error_type, error_message, timing_breakdown
		FROM check_results
		WHERE site_id = $1
		ORDER BY checked_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, siteID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*CheckResult
	for rows.Next() {
		var r CheckResult
		var rawTiming []byte
		if err := rows.Scan(
			&r.ID, &r.SiteID, &r.CheckedAt, &r.StatusCode, &r.ResponseTimeMs,
			&r.IsUp, &r.ErrorType, &r.ErrorMessage, &rawTiming,
		); err != nil {
			return nil, err
		}
		if rawTiming != nil {
			r.TimingBreakdown = json.RawMessage(rawTiming)
		}
		results = append(results, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []*CheckResult{}
	}
	return results, nil
}

// UpdateSiteState updates or inserts the current live state for a site.
func (s *PostgresStore) UpdateSiteState(ctx context.Context, state *SiteState) error {
	if state == nil {
		return errors.New("site state cannot be nil")
	}

	query := `
		INSERT INTO site_state (
			site_id, current_status, consecutive_failures,
			last_checked_at, last_status_change_at, uptime_pct_24h, uptime_pct_30d
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (site_id) DO UPDATE SET
			current_status = EXCLUDED.current_status,
			consecutive_failures = EXCLUDED.consecutive_failures,
			last_checked_at = EXCLUDED.last_checked_at,
			last_status_change_at = EXCLUDED.last_status_change_at,
			uptime_pct_24h = EXCLUDED.uptime_pct_24h,
			uptime_pct_30d = EXCLUDED.uptime_pct_30d
	`
	_, err := s.db.ExecContext(
		ctx, query,
		state.SiteID, string(state.CurrentStatus), state.ConsecutiveFailures,
		state.LastCheckedAt, state.LastStatusChangeAt, state.UptimePct24h, state.UptimePct30d,
	)
	return err
}

// GetSiteState retrieves the current state of a site.
func (s *PostgresStore) GetSiteState(ctx context.Context, siteID uuid.UUID) (*SiteState, error) {
	query := `
		SELECT site_id, current_status, consecutive_failures,
		       last_checked_at, last_status_change_at, uptime_pct_24h, uptime_pct_30d
		FROM site_state
		WHERE site_id = $1
	`
	var state SiteState
	var statusStr string
	err := s.db.QueryRowContext(ctx, query, siteID).Scan(
		&state.SiteID, &statusStr, &state.ConsecutiveFailures,
		&state.LastCheckedAt, &state.LastStatusChangeAt, &state.UptimePct24h, &state.UptimePct30d,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSiteStateNotFound
		}
		return nil, err
	}
	state.CurrentStatus = SiteStatus(statusStr)
	return &state, nil
}

// CountCheckResults returns total number of recorded check results for a site.
func (s *PostgresStore) CountCheckResults(ctx context.Context, siteID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM check_results WHERE site_id = $1`
	var count int
	err := s.db.QueryRowContext(ctx, query, siteID).Scan(&count)
	return count, err
}

// GetSiteUptimeStats calculates availability percentages and response metrics for 24h, 7d, and 30d windows.
func (s *PostgresStore) GetSiteUptimeStats(ctx context.Context, siteID uuid.UUID) (*SiteUptimeStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE checked_at > NOW() - INTERVAL '24 hours') AS total_24h,
			COUNT(*) FILTER (WHERE checked_at > NOW() - INTERVAL '24 hours' AND is_up = true) AS up_24h,
			COALESCE(AVG(response_time_ms) FILTER (WHERE checked_at > NOW() - INTERVAL '24 hours' AND response_time_ms > 0), 0) AS avg_resp_24h,
			COUNT(*) FILTER (WHERE checked_at > NOW() - INTERVAL '7 days') AS total_7d,
			COUNT(*) FILTER (WHERE checked_at > NOW() - INTERVAL '7 days' AND is_up = true) AS up_7d,
			COUNT(*) FILTER (WHERE checked_at > NOW() - INTERVAL '30 days') AS total_30d,
			COUNT(*) FILTER (WHERE checked_at > NOW() - INTERVAL '30 days' AND is_up = true) AS up_30d
		FROM check_results
		WHERE site_id = $1
	`
	var total24h, up24h, total7d, up7d, total30d, up30d int
	var avgResp24h float64

	err := s.db.QueryRowContext(ctx, query, siteID).Scan(
		&total24h, &up24h, &avgResp24h,
		&total7d, &up7d,
		&total30d, &up30d,
	)
	if err != nil {
		return nil, err
	}

	calcPct := func(up, total int) float64 {
		if total == 0 {
			return 100.0
		}
		raw := (float64(up) / float64(total)) * 100.0
		return math.Round(raw*100) / 100
	}

	return &SiteUptimeStats{
		SiteID:               siteID,
		UptimePct24h:         calcPct(up24h, total24h),
		UptimePct7d:          calcPct(up7d, total7d),
		UptimePct30d:         calcPct(up30d, total30d),
		AvgResponseTimeMs24h: int(math.Round(avgResp24h)),
		TotalChecks24h:       total24h,
		TotalChecks7d:        total7d,
		TotalChecks30d:       total30d,
	}, nil
}

// GetDashboardSummary aggregates status counts and average 24h uptime for a user's active sites.
func (s *PostgresStore) GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*DashboardSummary, error) {
	query := `
		SELECT
			COUNT(s.id) AS total_sites,
			COUNT(s.id) FILTER (WHERE COALESCE(st.current_status, 'pending') = 'up') AS up_count,
			COUNT(s.id) FILTER (WHERE st.current_status = 'down') AS down_count,
			COUNT(s.id) FILTER (WHERE st.current_status = 'up' AND st.consecutive_failures > 0) AS degraded_count,
			COUNT(s.id) FILTER (WHERE COALESCE(st.current_status, 'pending') = 'pending') AS pending_count
		FROM sites s
		LEFT JOIN site_state st ON st.site_id = s.id
		WHERE s.user_id = $1 AND s.deleted_at IS NULL
	`

	var summary DashboardSummary
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&summary.TotalSites,
		&summary.UpCount,
		&summary.DownCount,
		&summary.DegradedCount,
		&summary.PendingCount,
	)
	if err != nil {
		return nil, err
	}

	uptimeQuery := `
		SELECT
			COALESCE(
				AVG(
					CASE WHEN site_checks.total > 0
					     THEN (site_checks.up::float / site_checks.total::float) * 100.0
					     ELSE NULL
					END
				),
				100.0
			)
		FROM sites s
		JOIN LATERAL (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE is_up = true) AS up
			FROM check_results cr
			WHERE cr.site_id = s.id AND cr.checked_at > NOW() - INTERVAL '24 hours'
		) site_checks ON true
		WHERE s.user_id = $1 AND s.deleted_at IS NULL
	`
	var avgUptime float64
	err = s.db.QueryRowContext(ctx, uptimeQuery, userID).Scan(&avgUptime)
	if err != nil {
		return nil, err
	}
	summary.AvgUptimePct24h = math.Round(avgUptime*100) / 100

	return &summary, nil
}


func (s *PostgresStore) attachTagsTx(ctx context.Context, tx *sql.Tx, userID, siteID uuid.UUID, tags []string) ([]string, error) {
	var normalized []string
	seen := make(map[string]bool)

	for _, raw := range tags {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		normalized = append(normalized, name)

		// Insert or get tag
		var tagID uuid.UUID
		tagQuery := `
			INSERT INTO tags (id, user_id, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`
		err := tx.QueryRowContext(ctx, tagQuery, uuid.New(), userID, name).Scan(&tagID)
		if err != nil {
			return nil, err
		}

		// Link in site_tags
		linkQuery := `
			INSERT INTO site_tags (site_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`
		if _, err := tx.ExecContext(ctx, linkQuery, siteID, tagID); err != nil {
			return nil, err
		}
	}

	if normalized == nil {
		normalized = []string{}
	}
	return normalized, nil
}

func (s *PostgresStore) getSiteTags(ctx context.Context, siteID uuid.UUID) ([]string, error) {
	query := `
		SELECT t.name
		FROM tags t
		JOIN site_tags st ON st.tag_id = t.id
		WHERE st.site_id = $1
		ORDER BY t.name ASC
	`
	rows, err := s.db.QueryContext(ctx, query, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, rows.Err()
}

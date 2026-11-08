package store

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore provides an in-memory implementation of the Store interface for testing.
type MemoryStore struct {
	mu            sync.RWMutex
	users         map[uuid.UUID]*User
	refreshTokens map[string]*RefreshToken
	sites         map[uuid.UUID]*Site
	siteStates    map[uuid.UUID]*SiteState
	tags          map[uuid.UUID]*Tag
	siteTags      map[uuid.UUID][]uuid.UUID
	checkResults  map[uuid.UUID][]*CheckResult
	nextCheckID   int64
	channels      map[uuid.UUID]*NotificationChannel
	events        []*NotificationEvent
}

// NewMemoryStore creates a new in-memory store instance.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:         make(map[uuid.UUID]*User),
		refreshTokens: make(map[string]*RefreshToken),
		sites:         make(map[uuid.UUID]*Site),
		siteStates:    make(map[uuid.UUID]*SiteState),
		tags:          make(map[uuid.UUID]*Tag),
		siteTags:      make(map[uuid.UUID][]uuid.UUID),
		checkResults:  make(map[uuid.UUID][]*CheckResult),
		nextCheckID:   1,
		channels:      make(map[uuid.UUID]*NotificationChannel),
		events:        make([]*NotificationEvent, 0),
	}
}

// Ping verifies that the memory store is operational.
func (m *MemoryStore) Ping(_ context.Context) error {
	return nil
}

// CreateUser creates a new user in memory.
func (m *MemoryStore) CreateUser(_ context.Context, user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))

	// Check unique email constraint
	for _, u := range m.users {
		if strings.ToLower(u.Email) == normalizedEmail {
			return ErrDuplicateEmail
		}
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
	user.Email = normalizedEmail

	clone := *user
	m.users[user.ID] = &clone

	return nil
}

// GetUserByEmail finds a user by email in memory.
func (m *MemoryStore) GetUserByEmail(_ context.Context, email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, u := range m.users {
		if strings.ToLower(u.Email) == normalizedEmail {
			clone := *u
			return &clone, nil
		}
	}

	return nil, ErrUserNotFound
}

// GetUserByID finds a user by ID in memory.
func (m *MemoryStore) GetUserByID(_ context.Context, id uuid.UUID) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	clone := *u
	return &clone, nil
}

// CreateRefreshToken stores a new refresh token in memory.
func (m *MemoryStore) CreateRefreshToken(_ context.Context, token *RefreshToken) error {
	if token == nil {
		return errors.New("refresh token cannot be nil")
	}
	if token.TokenHash == "" {
		return errors.New("token hash cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	clone := *token
	m.refreshTokens[token.TokenHash] = &clone

	return nil
}

// GetRefreshToken retrieves a refresh token by its hash.
func (m *MemoryStore) GetRefreshToken(_ context.Context, tokenHash string) (*RefreshToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.refreshTokens[tokenHash]
	if !ok {
		return nil, ErrRefreshTokenNotFound
	}

	clone := *token
	return &clone, nil
}

// RevokeRefreshToken marks a specific refresh token as revoked.
func (m *MemoryStore) RevokeRefreshToken(_ context.Context, tokenHash string, revokedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.refreshTokens[tokenHash]
	if !ok {
		return ErrRefreshTokenNotFound
	}

	token.RevokedAt = &revokedAt
	return nil
}

// RevokeUserRefreshTokens revokes all active refresh tokens belonging to a user.
func (m *MemoryStore) RevokeUserRefreshTokens(_ context.Context, userID uuid.UUID, revokedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, token := range m.refreshTokens {
		if token.UserID == userID && token.RevokedAt == nil {
			token.RevokedAt = &revokedAt
		}
	}

	return nil
}

// CreateSite creates a new site, initializes its state, and attaches any tags.
func (m *MemoryStore) CreateSite(_ context.Context, site *Site, tags []string) (*SiteWithTags, error) {
	if site == nil {
		return nil, errors.New("site cannot be nil")
	}
	if site.UserID == uuid.Nil {
		return nil, errors.New("site must have a valid user_id")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check active duplicate normalized URL for user
	for _, s := range m.sites {
		if s.UserID == site.UserID && s.NormalizedURL == site.NormalizedURL && s.DeletedAt == nil {
			return nil, ErrDuplicateSite
		}
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

	clone := *site
	m.sites[site.ID] = &clone

	// Initialize site_state
	m.siteStates[site.ID] = &SiteState{
		SiteID:              site.ID,
		CurrentStatus:       SiteStatusPending,
		ConsecutiveFailures: 0,
	}

	// Process tags
	normalizedTags := m.attachTagsLocked(site.UserID, site.ID, tags)

	return &SiteWithTags{
		Site:          clone,
		Tags:          normalizedTags,
		CurrentStatus: SiteStatusPending,
	}, nil
}

// GetSiteByID fetches an active site by ID for the specified user.
func (m *MemoryStore) GetSiteByID(_ context.Context, userID, siteID uuid.UUID) (*SiteWithTags, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sites[siteID]
	if !ok || s.UserID != userID || s.DeletedAt != nil {
		return nil, ErrSiteNotFound
	}

	status := SiteStatusPending
	if state, ok := m.siteStates[siteID]; ok {
		status = state.CurrentStatus
	}

	tagNames := m.getSiteTagNamesLocked(siteID)

	clone := *s
	return &SiteWithTags{
		Site:          clone,
		Tags:          tagNames,
		CurrentStatus: status,
	}, nil
}

// ListSites returns all active sites for a user, optionally filtered by tag.
func (m *MemoryStore) ListSites(_ context.Context, userID uuid.UUID, tagFilter string) ([]*SiteWithTags, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SiteWithTags
	tagFilter = strings.TrimSpace(tagFilter)

	for _, s := range m.sites {
		if s.UserID != userID || s.DeletedAt != nil {
			continue
		}

		tagNames := m.getSiteTagNamesLocked(s.ID)

		if tagFilter != "" {
			matches := false
			for _, t := range tagNames {
				if strings.EqualFold(t, tagFilter) {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
		}

		status := SiteStatusPending
		if state, ok := m.siteStates[s.ID]; ok {
			status = state.CurrentStatus
		}

		clone := *s
		result = append(result, &SiteWithTags{
			Site:          clone,
			Tags:          tagNames,
			CurrentStatus: status,
		})
	}

	// Sort descending by CreatedAt
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// UpdateSite updates specified properties of an existing active site.
func (m *MemoryStore) UpdateSite(_ context.Context, userID, siteID uuid.UUID, input UpdateSiteInput) (*SiteWithTags, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sites[siteID]
	if !ok || s.UserID != userID || s.DeletedAt != nil {
		return nil, ErrSiteNotFound
	}

	if input.Label != nil {
		s.Label = input.Label
	}
	if input.CheckIntervalSec != nil {
		s.CheckIntervalSec = *input.CheckIntervalSec
	}
	if input.TimeoutSec != nil {
		s.TimeoutSec = *input.TimeoutSec
	}
	if input.FailureThreshold != nil {
		s.FailureThreshold = *input.FailureThreshold
	}
	if input.RetryCount != nil {
		s.RetryCount = *input.RetryCount
	}
	if input.ExpectedStatusMin != nil {
		s.ExpectedStatusMin = *input.ExpectedStatusMin
	}
	if input.ExpectedStatusMax != nil {
		s.ExpectedStatusMax = *input.ExpectedStatusMax
	}

	if input.Tags != nil {
		delete(m.siteTags, siteID)
		m.attachTagsLocked(userID, siteID, *input.Tags)
	}

	tagNames := m.getSiteTagNamesLocked(siteID)
	status := SiteStatusPending
	if state, ok := m.siteStates[siteID]; ok {
		status = state.CurrentStatus
	}

	clone := *s
	return &SiteWithTags{
		Site:          clone,
		Tags:          tagNames,
		CurrentStatus: status,
	}, nil
}

// DeleteSite soft-deletes a site.
func (m *MemoryStore) DeleteSite(_ context.Context, userID, siteID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sites[siteID]
	if !ok || s.UserID != userID || s.DeletedAt != nil {
		return ErrSiteNotFound
	}

	now := time.Now().UTC()
	s.DeletedAt = &now
	return nil
}

// GetAllActiveSites returns all non-deleted sites across all users.
func (m *MemoryStore) GetAllActiveSites(_ context.Context) ([]*Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Site
	for _, s := range m.sites {
		if s.DeletedAt == nil {
			clone := *s
			result = append(result, &clone)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

// RecordCheckResult records an individual check result.
func (m *MemoryStore) RecordCheckResult(_ context.Context, result *CheckResult) error {
	if result == nil {
		return errors.New("check result cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if result.ID == 0 {
		result.ID = m.nextCheckID
		m.nextCheckID++
	}
	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now().UTC()
	}

	clone := *result
	m.checkResults[result.SiteID] = append(m.checkResults[result.SiteID], &clone)
	return nil
}

// GetCheckResults returns check results for a site ordered newest first.
func (m *MemoryStore) GetCheckResults(_ context.Context, siteID uuid.UUID, limit, offset int) ([]*CheckResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, ok := m.checkResults[siteID]
	if !ok || len(history) == 0 {
		return []*CheckResult{}, nil
	}

	// Copy and sort descending
	sorted := make([]*CheckResult, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CheckedAt.After(sorted[j].CheckedAt)
	})

	if offset < 0 {
		offset = 0
	}
	if offset >= len(sorted) {
		return []*CheckResult{}, nil
	}

	end := len(sorted)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	sliced := sorted[offset:end]
	res := make([]*CheckResult, len(sliced))
	for i, c := range sliced {
		clone := *c
		res[i] = &clone
	}
	return res, nil
}

// UpdateSiteState updates or creates the current state for a site.
func (m *MemoryStore) UpdateSiteState(_ context.Context, state *SiteState) error {
	if state == nil {
		return errors.New("site state cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	clone := *state
	m.siteStates[state.SiteID] = &clone
	return nil
}

// GetSiteState retrieves the current state of a site.
func (m *MemoryStore) GetSiteState(_ context.Context, siteID uuid.UUID) (*SiteState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.siteStates[siteID]
	if !ok {
		return nil, ErrSiteStateNotFound
	}

	clone := *state
	return &clone, nil
}

// CountCheckResults returns the total number of recorded check results for a site.
func (m *MemoryStore) CountCheckResults(_ context.Context, siteID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.checkResults[siteID]), nil
}

// GetSiteUptimeStats calculates availability percentages and response metrics for 24h, 7d, and 30d windows.
func (m *MemoryStore) GetSiteUptimeStats(_ context.Context, siteID uuid.UUID) (*SiteUptimeStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.checkResults[siteID]
	now := time.Now().UTC()
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff7d := now.Add(-7 * 24 * time.Hour)
	cutoff30d := now.Add(-30 * 24 * time.Hour)

	var total24h, up24h int
	var total7d, up7d int
	var total30d, up30d int
	var sumResponseTime24h, countResponseTime24h int

	for _, c := range history {
		if c.CheckedAt.After(cutoff24h) {
			total24h++
			if c.IsUp {
				up24h++
			}
			if c.ResponseTimeMs != nil && *c.ResponseTimeMs > 0 {
				sumResponseTime24h += *c.ResponseTimeMs
				countResponseTime24h++
			}
		}
		if c.CheckedAt.After(cutoff7d) {
			total7d++
			if c.IsUp {
				up7d++
			}
		}
		if c.CheckedAt.After(cutoff30d) {
			total30d++
			if c.IsUp {
				up30d++
			}
		}
	}

	calcPct := func(up, total int) float64 {
		if total == 0 {
			return 100.0
		}
		raw := (float64(up) / float64(total)) * 100.0
		return math.Round(raw*100) / 100
	}

	avgResp := 0
	if countResponseTime24h > 0 {
		avgResp = sumResponseTime24h / countResponseTime24h
	}

	return &SiteUptimeStats{
		SiteID:               siteID,
		UptimePct24h:         calcPct(up24h, total24h),
		UptimePct7d:          calcPct(up7d, total7d),
		UptimePct30d:         calcPct(up30d, total30d),
		AvgResponseTimeMs24h: avgResp,
		TotalChecks24h:       total24h,
		TotalChecks7d:        total7d,
		TotalChecks30d:       total30d,
	}, nil
}

// GetDashboardSummary aggregates site counts and status breakdown for a user.
func (m *MemoryStore) GetDashboardSummary(_ context.Context, userID uuid.UUID) (*DashboardSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &DashboardSummary{}
	now := time.Now().UTC()
	cutoff24h := now.Add(-24 * time.Hour)

	var uptimeSum float64
	var sitesWithChecks int

	for _, site := range m.sites {
		if site.UserID != userID || site.DeletedAt != nil {
			continue
		}
		summary.TotalSites++

		state := m.siteStates[site.ID]
		if state == nil {
			summary.PendingCount++
		} else {
			switch state.CurrentStatus {
			case SiteStatusUp:
				if state.ConsecutiveFailures > 0 {
					summary.DegradedCount++
				}
				summary.UpCount++
			case SiteStatusDown:
				summary.DownCount++
			default:
				summary.PendingCount++
			}
		}

		// Calculate 24h uptime for this site
		history := m.checkResults[site.ID]
		var total, up int
		for _, c := range history {
			if c.CheckedAt.After(cutoff24h) {
				total++
				if c.IsUp {
					up++
				}
			}
		}
		if total > 0 {
			uptimeSum += (float64(up) / float64(total)) * 100.0
			sitesWithChecks++
		}
	}

	if sitesWithChecks > 0 {
		raw := uptimeSum / float64(sitesWithChecks)
		summary.AvgUptimePct24h = math.Round(raw*100) / 100
	} else {
		summary.AvgUptimePct24h = 100.0
	}

	return summary, nil
}

// Helper methods (must be called with lock held)
func (m *MemoryStore) attachTagsLocked(userID, siteID uuid.UUID, tags []string) []string {
	var normalizedTagNames []string
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
		normalizedTagNames = append(normalizedTagNames, name)

		// Find or create tag
		var tagID uuid.UUID
		for _, t := range m.tags {
			if t.UserID == userID && strings.EqualFold(t.Name, name) {
				tagID = t.ID
				break
			}
		}

		if tagID == uuid.Nil {
			tagID = uuid.New()
			m.tags[tagID] = &Tag{
				ID:     tagID,
				UserID: userID,
				Name:   name,
			}
		}

		m.siteTags[siteID] = append(m.siteTags[siteID], tagID)
	}

	return normalizedTagNames
}

func (m *MemoryStore) getSiteTagNamesLocked(siteID uuid.UUID) []string {
	tagIDs := m.siteTags[siteID]
	var names []string
	for _, id := range tagIDs {
		if t, ok := m.tags[id]; ok {
			names = append(names, t.Name)
		}
	}
	if names == nil {
		names = []string{}
	}
	return names
}

// CreateNotificationChannel creates a notification target channel.
func (m *MemoryStore) CreateNotificationChannel(_ context.Context, ch *NotificationChannel) error {
	if ch == nil {
		return errors.New("channel cannot be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch.ID == uuid.Nil {
		ch.ID = uuid.New()
	}
	if ch.CreatedAt.IsZero() {
		ch.CreatedAt = time.Now().UTC()
	}

	clone := *ch
	m.channels[ch.ID] = &clone
	return nil
}

// GetNotificationChannelByID retrieves a channel by ID verifying user ownership.
func (m *MemoryStore) GetNotificationChannelByID(_ context.Context, userID, channelID uuid.UUID) (*NotificationChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[channelID]
	if !ok || ch.UserID != userID {
		return nil, ErrChannelNotFound
	}

	clone := *ch
	return &clone, nil
}

// ListNotificationChannels returns all notification channels for a user, optionally filtered by site.
func (m *MemoryStore) ListNotificationChannels(_ context.Context, userID uuid.UUID, siteID *uuid.UUID) ([]*NotificationChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*NotificationChannel
	for _, ch := range m.channels {
		if ch.UserID != userID {
			continue
		}
		if siteID != nil {
			if ch.SiteID == nil || *ch.SiteID != *siteID {
				continue
			}
		}
		clone := *ch
		result = append(result, &clone)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

// UpdateNotificationChannel updates the enabled status and/or target of a channel.
func (m *MemoryStore) UpdateNotificationChannel(_ context.Context, userID, channelID uuid.UUID, isEnabled *bool, target *string) (*NotificationChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[channelID]
	if !ok || ch.UserID != userID {
		return nil, ErrChannelNotFound
	}

	if isEnabled != nil {
		ch.IsEnabled = *isEnabled
	}
	if target != nil {
		ch.Target = *target
	}

	clone := *ch
	return &clone, nil
}

// DeleteNotificationChannel removes a channel for a user.
func (m *MemoryStore) DeleteNotificationChannel(_ context.Context, userID, channelID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[channelID]
	if !ok || ch.UserID != userID {
		return ErrChannelNotFound
	}

	delete(m.channels, channelID)
	return nil
}

// GetChannelsForSiteTransition returns all active notification channels applicable to a site.
func (m *MemoryStore) GetChannelsForSiteTransition(_ context.Context, site *Site) ([]*NotificationChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*NotificationChannel
	for _, ch := range m.channels {
		if ch.UserID == site.UserID && ch.IsEnabled {
			if ch.SiteID == nil || *ch.SiteID == site.ID {
				clone := *ch
				result = append(result, &clone)
			}
		}
	}
	return result, nil
}

// RecordNotificationEvent records a notification attempt (sent, failed, or suppressed).
func (m *MemoryStore) RecordNotificationEvent(_ context.Context, event *NotificationEvent) error {
	if event == nil {
		return errors.New("event cannot be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.SentAt.IsZero() {
		event.SentAt = time.Now().UTC()
	}

	clone := *event
	m.events = append(m.events, &clone)
	return nil
}

// GetLastNotificationEvent returns the most recent notification event for a site and channel.
func (m *MemoryStore) GetLastNotificationEvent(_ context.Context, siteID, channelID uuid.UUID) (*NotificationEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		if e.SiteID == siteID && e.ChannelID == channelID {
			clone := *e
			return &clone, nil
		}
	}
	return nil, nil
}

// ListNotificationEvents returns recent notification events for a user's sites.
func (m *MemoryStore) ListNotificationEvents(_ context.Context, userID uuid.UUID, limit, offset int) ([]*NotificationEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userSites := make(map[uuid.UUID]bool)
	for _, s := range m.sites {
		if s.UserID == userID {
			userSites[s.ID] = true
		}
	}

	var filtered []*NotificationEvent
	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		if userSites[e.SiteID] {
			clone := *e
			filtered = append(filtered, &clone)
		}
	}

	if offset < 0 {
		offset = 0
	}
	if offset >= len(filtered) {
		return []*NotificationEvent{}, nil
	}

	end := len(filtered)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return filtered[offset:end], nil
}


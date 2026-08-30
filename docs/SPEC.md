# Web Monitor Tech Spec

## Overview

Web Monitor is an application that lets users tag their favourite websites and track those sites' status (uptime, response time, and health) in a clean, at-a-glance dashboard.

Backend: Golang
Frontend: React
Core value prop: Add a URL -> Web Monitor checks it periodically-> user sees live status, history, and alerts in a dashboard.

## Requirements

Product Requirements
- Let users add/tag and organize websites they care about
- Periodically check each site's availability and performance
- (OPTIONAL) Present current and historical status in a dashboard
- (OPTIONAL) Notify users when a track site goes down or recovers

Technical Requirements
- Default check types: HTTPS GET with configurable expected status codes (200-399)
- Configurable timeout on check (e.g. 10s default)
- Retry policy: on failure, one retry before marking as failed (configurable with configuration)
- Store rolled-up status for faster dashboard reads
- Trigger notifications only on state transitions (up->down, down->up)
- Channels (v1) email, (v2) Webhook, Push
- Debounce/rate-limit to avoid notification storms

## High Level Architecture

React/Frontend <----- HTTP/REST (+WebSocket) ---->  Go API Server

API Server: stateless Go service exposing a REST API and optionally WebSocket endpoint for live dashboard updates
Scheduler/Worker pool: background Go routines that periodically ping each site and writes to DB
Database: PostgreSQL for persistentce (sites to monitor, users, check history)
Notifications: asyn work that reacts to state changes (ping tab while open / maybe email user)

## Backend (Golang)

```
/cmd
   /api       # main enytpoint for API server
   /worker    # main entrypoint for scheduler process
/internal
   /auth      # auth middleware. JWT/session handling
   /sites     # sites CRUD and logic
   /checker   # HTTP probes logic
   /notify    # notification dispatch logic
   /store     # DB access layer
   /api       # HTTP handlers, routing
```

### Key Components

- Each monitored site has a check_interval (e.g. 30s, 1m, 5m)
- A scheduler (in-process ticker map, or a job queue like river/asynq backed by Postgres/Redis) triggers checks
- A worker pool (bounded goroutines) performs the actual HTTP request:
  - Records: status code, response time, DNS/connect/TLS timing (optional), error (if any)
  - Applies a "failure threshold" (e.g. 2 consecutive failures) before marking a site "down," to avoid false positives from transient blips

### Suggested Libraries
- Router: net/http
- DB access: sqlc or pgx directly
- Migrations: golang-migrate
- Scheduling: simple time.Ticker per site for v1; move to a queue-based system if scaling beyond a few thousand sites
- Config: godotenv
- Auth: golang-jwt/jwt

### Data Models

-- USERS
users
    id                  uuid pk
    email               text unique not null
    password_hash       text not null
    timezone            text default 'UTC'
    email_verified_at   timestamptz
    created_at          timestamptz not null default now()

-- SITES (config only — no live status)
sites
    id                  uuid pk
    user_id             uuid fk -> users.id
    url                 text not null              -- as entered, for display
    normalized_url      text not null               -- for dedup/uniqueness
    label               text
    check_interval_sec  int not null default 60
    timeout_sec         int not null default 10
    failure_threshold   int not null default 2      -- consecutive fails before "down"
    retry_count         int not null default 1
    expected_status_min int not null default 200
    expected_status_max int not null default 399
    created_at          timestamptz not null default now()
    deleted_at          timestamptz                 -- soft delete

    unique (user_id, normalized_url) where deleted_at is null

-- TAGS (normalized, not array)
tags
    id          uuid pk
    user_id     uuid fk -> users.id
    name        text not null
    unique (user_id, name)

site_tags
    site_id     uuid fk -> sites.id
    tag_id      uuid fk -> tags.id
    primary key (site_id, tag_id)

-- CHECK RESULTS (time-series, append-only)
check_results
    id                  bigint pk               -- bigint, not uuid: high-volume, sequential-friendly
    site_id             uuid fk -> sites.id
    checked_at          timestamptz not null
    status_code         int
    response_time_ms    int
    is_up               boolean not null        -- outcome of THIS check, not site state
    error_type          text                    -- 'timeout', 'dns', 'tls', 'connection_refused', etc.
    error_message        text
    timing_breakdown    jsonb                   -- optional DNS/connect/TLS detail, v2-ready
    -- partition by range on checked_at (monthly), or move to a rollup table beyond N days

-- SITE STATE (live/current — single source of truth for "is it up right now")
site_state
    site_id                 uuid pk fk -> sites.id
    current_status          text not null check (current_status in ('up','down','pending'))
    consecutive_failures    int not null default 0
    last_checked_at         timestamptz
    last_status_change_at   timestamptz
    uptime_pct_24h          numeric(5,2)         -- denormalized/cached, refreshed by a job
    uptime_pct_30d          numeric(5,2)

-- NOTIFICATION PREFERENCES (per-user config)
notification_channels
    id          uuid pk
    user_id     uuid fk -> users.id
    site_id     uuid fk -> sites.id nullable     -- null = applies to all sites
    type        text not null                    -- 'email' | 'webhook' | 'push'
    target      text not null                    -- email address / webhook URL / push token
    is_enabled  boolean not null default true
    created_at  timestamptz not null default now()

-- NOTIFICATION LOG (sent notifications — for debounce + history UI)
notification_events
    id              uuid pk
    site_id         uuid fk -> sites.id
    channel_id      uuid fk -> notification_channels.id
    transition      text not null      -- 'up_to_down' | 'down_to_up'
    sent_at         timestamptz not null default now()
    delivery_status text                -- 'sent' | 'failed' | 'suppressed_debounce'


### API

POST /api/auth/register     Create account
POST /api/auth/login        Login, returns JWT

GET /api/sites              List user's montiored sites
POST /api/sites             Add a new site ot monitor
PUT /api/sites/:id          Update label, tags, check interval
DELETE /api/sites/:id        Stop monitoring a site

GET /api/sites/:id/history  Historical check results
GET /api/sites/:id/uptime   Uptime stats

WS /api/ws/status           Live status updates

## Frontend (React)

```
/src
    /api
    /components
    /pages
    /hooks
    /context
```

### Key Screens

- Dashboard: grid/list of tracked sites, each with a status badge (green/yellow/red), response time sparkline, and tags; filterable by tag/status
- Site Detail: uptime % over 24h/7d/30d, response-time chart over time, incident history log
- Add/Edit Site: URL, friendly label, tags, check interval, notification preferences
- Settings: notification channels (email/webhook), account info


### Suggested Libraries
- Charting: recharts
- WebSockets

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
- Router: chi or gin
- DB access: sqlc or pgx directly (avoid heavy ORM for performance/clarity)
- Migrations: golang-migrate
- Scheduling: simple time.Ticker per site for v1; move to a queue-based system if scaling beyond a few thousand sites
- Config: envconfig or viper
- Auth: JWT via golang-jwt/jwt

### Data Models

users
    id, email, password_hash, created_at

sites
    id, user_id (fk), url, label, tags text[], is_up

check_results
   id, site_id (fk), check_at, status_code, response_time_ms, is_up

site_state
    site_id, current_status, last_checked_at, last_status_change_at, uptime_pct_24h, uptime_pct_30d

notifications
   id, user_id, site_id, type, target, created_at

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

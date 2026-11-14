# Web Monitor Backend API Documentation

The Web Monitor Backend API is a RESTful service written in Go for configuring websites, scheduling uptime probes, aggregating health/latency metrics, and dispatching alerts.

- **Default Base URL:** `http://localhost:8080`
- **Default Content-Type:** `application/json`
- **CORS:** Enabled (`*`) with support for standard HTTP methods (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`).

---

## Table of Contents

1. [Authentication & Authorization](#authentication--authorization)
2. [Rate Limiting](#rate-limiting)
3. [Error Handling](#error-handling)
4. [System Endpoints](#system-endpoints)
   - [Health Check](#health-check)
5. [Authentication Endpoints](#authentication-endpoints)
   - [Register](#register)
   - [Login](#login)
   - [Refresh Token](#refresh-token)
   - [Current User Profile](#current-user-profile)
6. [Dashboard Endpoints](#dashboard-endpoints)
   - [Dashboard Summary](#dashboard-summary)
7. [Sites Management Endpoints](#sites-management-endpoints)
   - [Create Site](#create-site)
   - [List Sites](#list-sites)
   - [Get Site Details](#get-site-details)
   - [Update Site](#update-site)
   - [Delete Site](#delete-site)
8. [Metrics & History Endpoints](#metrics--history-endpoints)
   - [Site Uptime Statistics](#site-uptime-statistics)
   - [Site Check History](#site-check-history)
9. [Notification Channels Endpoints](#notification-channels-endpoints)
   - [Create Channel](#create-channel)
   - [List Channels](#list-channels)
   - [Update Channel](#update-channel)
   - [Delete Channel](#delete-channel)
   - [List Notification Events](#list-notification-events)
10. [Quickstart Walkthrough (cURL)](#quickstart-walkthrough-curl)

---

## Authentication & Authorization

All protected endpoints require an HTTP `Authorization` header containing a JSON Web Token (JWT) in Bearer format:

```http
Authorization: Bearer <your_access_token>
```

- **Access Token Expiration:** 24 hours
- **Refresh Token Expiration:** 7 days

When an access token expires, client applications can obtain a new access and refresh token pair using the `POST /api/auth/refresh` endpoint without prompting the user to re-enter credentials.

---

## Rate Limiting

Sensitive authentication endpoints (`/api/auth/register`, `/api/auth/login`, and `/api/auth/refresh`) are protected by an in-memory token-bucket rate limiter:

- **Limit:** 60 requests per minute per client IP
- **Exceeded Response:** `429 Too Many Requests`

```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

---

## Error Handling

All error responses return a standardized JSON format:

```json
{
  "error": "Descriptive error message"
}
```

### Standard Status Codes

| Code | Meaning | Typical Trigger |
| :--- | :--- | :--- |
| `200` | OK | Successful request |
| `201` | Created | Resource successfully created |
| `400` | Bad Request | Invalid request body, malformed JSON, or validation failure |
| `401` | Unauthorized | Missing, invalid, or expired JWT token / credentials |
| `404` | Not Found | Requested site or channel does not exist or belongs to another user |
| `409` | Conflict | User email or monitored URL already registered |
| `415` | Unsupported Media Type | Request body sent without `Content-Type: application/json` |
| `429` | Too Many Requests | Rate limit threshold exceeded |
| `500` | Internal Server Error | Unexpected server-side failure |
| `503` | Service Unavailable | Database or underlying service is unreachable |

---

## System Endpoints

### Health Check

Checks service health and underlying database connectivity. Used by container orchestrators and load balancers.

- **Method:** `GET`
- **Path:** `/api/health`
- **Auth Required:** No

#### Response (`200 OK`)

```json
{
  "status": "healthy",
  "message": "OK",
  "timestamp": "2026-11-14T21:58:39.522Z",
  "uptime": "31s",
  "services": {
    "database": "healthy"
  }
}
```

#### Unhealthy Response (`503 Service Unavailable`)

```json
{
  "status": "unhealthy",
  "message": "One or more dependencies are unhealthy",
  "timestamp": "2026-11-14T21:58:39.522Z",
  "uptime": "31s",
  "services": {
    "database": "unhealthy: dial tcp 172.18.0.2:5432: connect: connection refused"
  }
}
```

---

## Authentication Endpoints

### Register

Creates a new user account.

- **Method:** `POST`
- **Path:** `/api/auth/register`
- **Auth Required:** No
- **Headers:** `Content-Type: application/json`

#### Request Body

| Field | Type | Required | Description | Constraints |
| :--- | :--- | :--- | :--- | :--- |
| `email` | string | Yes | User email address | Valid email format, unique |
| `password` | string | Yes | Account password | 8 to 72 characters |
| `timezone` | string | No | User's preferred timezone | Defaults to `"UTC"` |

```json
{
  "email": "developer@example.com",
  "password": "SecurePassword123!",
  "timezone": "America/New_York"
}
```

#### Response (`201 Created`)

```json
{
  "user": {
    "id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "email": "developer@example.com",
    "timezone": "America/New_York",
    "created_at": "2026-11-14T22:00:00Z"
  },
  "message": "User registered successfully"
}
```

---

### Login

Authenticates credentials and returns JWT access and refresh tokens.

- **Method:** `POST`
- **Path:** `/api/auth/login`
- **Auth Required:** No
- **Headers:** `Content-Type: application/json`

#### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `email` | string | Yes | Registered account email |
| `password` | string | Yes | Plaintext password |

```json
{
  "email": "developer@example.com",
  "password": "SecurePassword123!"
}
```

#### Response (`200 OK`)

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "a8f3b723-92f7-43cf-bf24-2c67dbef912d",
  "user": {
    "id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "email": "developer@example.com",
    "timezone": "America/New_York",
    "created_at": "2026-11-14T22:00:00Z"
  },
  "expires_in": 86400
}
```

---

### Refresh Token

Rotates the current refresh token and issues a new access token.

- **Method:** `POST`
- **Path:** `/api/auth/refresh`
- **Auth Required:** No
- **Headers:** `Content-Type: application/json`

#### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `refresh_token` | string | Yes | Valid non-expired refresh token |

```json
{
  "refresh_token": "a8f3b723-92f7-43cf-bf24-2c67dbef912d"
}
```

#### Response (`200 OK`)

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "user": {
    "id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "email": "developer@example.com",
    "timezone": "America/New_York",
    "created_at": "2026-11-14T22:00:00Z"
  },
  "expires_in": 86400
}
```

---

### Current User Profile

Retrieves profile data for the authenticated user.

- **Method:** `GET`
- **Path:** `/api/auth/me`
- **Auth Required:** Yes (`Bearer <token>`)

#### Response (`200 OK`)

```json
{
  "id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
  "email": "developer@example.com",
  "timezone": "America/New_York",
  "email_verified_at": null,
  "created_at": "2026-11-14T22:00:00Z"
}
```

---

## Dashboard Endpoints

### Dashboard Summary

Fetches high-level status statistics across all monitored sites for the authenticated user.

- **Method:** `GET`
- **Path:** `/api/dashboard/summary`
- **Auth Required:** Yes (`Bearer <token>`)

#### Response (`200 OK`)

```json
{
  "total_sites": 6,
  "up_count": 5,
  "down_count": 0,
  "degraded_count": 0,
  "pending_count": 1,
  "avg_uptime_pct_24h": 99.85
}
```

---

## Sites Management Endpoints

### Create Site

Adds a new website for automated uptime and latency monitoring.

- **Method:** `POST`
- **Path:** `/api/sites`
- **Auth Required:** Yes (`Bearer <token>`)
- **Headers:** `Content-Type: application/json`

#### Request Body

| Field | Type | Required | Default | Constraints | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `url` | string | Yes | - | Valid HTTP/HTTPS URL | URL to probe (auto-prepends `https://` if scheme is omitted) |
| `label` | string | No | `null` | Max 100 chars | Human-readable site name |
| `check_interval_sec`| integer| No | `60` | `10` to `86400` | Probe frequency in seconds |
| `timeout_sec` | integer| No | `10` | `1` to `60` | HTTP request timeout |
| `failure_threshold` | integer| No | `2` | `1` to `10` | Consecutive failures before marking status as `"down"` |
| `retry_count` | integer| No | `1` | `0` to `5` | In-flight retries before recording a failed probe |
| `expected_status_min`| integer| No | `200` | `100` to `599` | Minimum accepted HTTP status code (inclusive) |
| `expected_status_max`| integer| No | `399` | `399` | Maximum accepted HTTP status code (inclusive) |
| `tags` | array[string] | No | `[]` | - | Descriptive labels for grouping |

```json
{
  "url": "https://api.github.com",
  "label": "GitHub API",
  "check_interval_sec": 30,
  "timeout_sec": 5,
  "failure_threshold": 3,
  "retry_count": 1,
  "expected_status_min": 200,
  "expected_status_max": 299,
  "tags": ["production", "external-api"]
}
```

#### Response (`201 Created`)

```json
{
  "site": {
    "id": "e2a4b578-8384-482d-8067-17ebaa619420",
    "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "url": "https://api.github.com",
    "normalized_url": "https://api.github.com",
    "label": "GitHub API",
    "check_interval_sec": 30,
    "timeout_sec": 5,
    "failure_threshold": 3,
    "retry_count": 1,
    "expected_status_min": 200,
    "expected_status_max": 299,
    "created_at": "2026-11-14T22:05:00Z",
    "tags": [
      "production",
      "external-api"
    ],
    "current_status": "pending"
  }
}
```

---

### List Sites

Retrieves all monitored sites owned by the authenticated user, optionally filtered by tag.

- **Method:** `GET`
- **Path:** `/api/sites`
- **Auth Required:** Yes (`Bearer <token>`)

#### Query Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `tag` | string | No | Filter sites by tag name (e.g. `?tag=production`) |

#### Response (`200 OK`)

```json
{
  "sites": [
    {
      "id": "e2a4b578-8384-482d-8067-17ebaa619420",
      "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
      "url": "https://api.github.com",
      "normalized_url": "https://api.github.com",
      "label": "GitHub API",
      "check_interval_sec": 30,
      "timeout_sec": 5,
      "failure_threshold": 3,
      "retry_count": 1,
      "expected_status_min": 200,
      "expected_status_max": 299,
      "created_at": "2026-11-14T22:05:00Z",
      "tags": [
        "production",
        "external-api"
      ],
      "current_status": "up"
    }
  ]
}
```

---

### Get Site Details

Retrieves a single site by ID.

- **Method:** `GET`
- **Path:** `/api/sites/{id}`
- **Auth Required:** Yes (`Bearer <token>`)

#### Path Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `id` | UUID | Yes | Monitored site ID |

#### Response (`200 OK`)

```json
{
  "site": {
    "id": "e2a4b578-8384-482d-8067-17ebaa619420",
    "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "url": "https://api.github.com",
    "normalized_url": "https://api.github.com",
    "label": "GitHub API",
    "check_interval_sec": 30,
    "timeout_sec": 5,
    "failure_threshold": 3,
    "retry_count": 1,
    "expected_status_min": 200,
    "expected_status_max": 299,
    "created_at": "2026-11-14T22:05:00Z",
    "tags": [
      "production",
      "external-api"
    ],
    "current_status": "up"
  }
}
```

---

### Update Site

Updates configuration or tags for an existing monitored site. Supports both `PATCH` (partial updates) and `PUT`.

- **Method:** `PATCH` or `PUT`
- **Path:** `/api/sites/{id}`
- **Auth Required:** Yes (`Bearer <token>`)
- **Headers:** `Content-Type: application/json`

#### Request Body (All fields optional)

```json
{
  "label": "GitHub Public API",
  "check_interval_sec": 60,
  "timeout_sec": 10,
  "failure_threshold": 2,
  "retry_count": 1,
  "expected_status_min": 200,
  "expected_status_max": 399,
  "tags": ["production", "v2"]
}
```

#### Response (`200 OK`)

Returns the updated site object with `SiteResponse`:

```json
{
  "site": {
    "id": "e2a4b578-8384-482d-8067-17ebaa619420",
    "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "url": "https://api.github.com",
    "normalized_url": "https://api.github.com",
    "label": "GitHub Public API",
    "check_interval_sec": 60,
    "timeout_sec": 10,
    "failure_threshold": 2,
    "retry_count": 1,
    "expected_status_min": 200,
    "expected_status_max": 399,
    "created_at": "2026-11-14T22:05:00Z",
    "tags": ["production", "v2"],
    "current_status": "up"
  }
}
```

---

### Delete Site

Soft-deletes a monitored site and stops future checks.

- **Method:** `DELETE`
- **Path:** `/api/sites/{id}`
- **Auth Required:** Yes (`Bearer <token>`)

#### Response (`200 OK`)

```json
{
  "message": "Site deleted successfully"
}
```

---

## Metrics & History Endpoints

### Site Uptime Statistics

Returns calculated uptime availability percentages and check volume over 24-hour, 7-day, and 30-day rolling windows.

- **Method:** `GET`
- **Path:** `/api/sites/{id}/uptime`
- **Auth Required:** Yes (`Bearer <token>`)

#### Response (`200 OK`)

```json
{
  "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
  "uptime_pct_24h": 100.0,
  "uptime_pct_7d": 99.95,
  "uptime_pct_30d": 99.80,
  "avg_response_time_ms_24h": 142,
  "total_checks_24h": 1440,
  "total_checks_7d": 10080,
  "total_checks_30d": 43200
}
```

---

### Site Check History

Returns paginated historical check probe records, including HTTP status codes, latency, and detailed network timing breakdowns.

- **Method:** `GET`
- **Path:** `/api/sites/{id}/history`
- **Auth Required:** Yes (`Bearer <token>`)

#### Query Parameters

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `limit` | integer | `50` | Maximum number of results to return |
| `offset` | integer | `0` | Number of records to skip |

#### Response (`200 OK`)

```json
{
  "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
  "results": [
    {
      "id": 1024,
      "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
      "checked_at": "2026-11-14T22:10:00Z",
      "status_code": 200,
      "response_time_ms": 115,
      "is_up": true,
      "error_type": null,
      "error_message": null,
      "timing_breakdown": {
        "dns_lookup_ms": 12,
        "tcp_connection_ms": 23,
        "tls_handshake_ms": 35,
        "server_processing_ms": 42,
        "content_transfer_ms": 3,
        "total_ms": 115
      }
    }
  ],
  "total": 120,
  "limit": 50,
  "offset": 0
}
```

---

## Notification Channels Endpoints

Notification channels receive alerts when a monitored site changes state (`up -> down` or `down -> up`).

### Supported Channel Types

- `email`: Sends an email alert to the specified address.
- `webhook`: Delivers an HTTP POST webhook containing transition details.
- `push`: Sends a mobile push notification using a device token.

---

### Create Channel

Creates a notification channel. Channels can be global (apply to all sites) or scoped to a specific site.

- **Method:** `POST`
- **Path:** `/api/notifications`
- **Auth Required:** Yes (`Bearer <token>`)
- **Headers:** `Content-Type: application/json`

#### Request Body

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `type` | string | Yes | - | `"email"`, `"webhook"`, or `"push"` |
| `target` | string | Yes | - | Email address, Webhook URL (`http://` or `https://`), or Push Token |
| `site_id` | UUID | No | `null` | Scopes alerts to a single site; if omitted or `null`, alerts apply to all sites |
| `is_enabled` | boolean| No | `true` | Whether the channel is active |

#### Webhook Channel Example

```json
{
  "type": "webhook",
  "target": "https://hooks.slack.com/services/T00/B00/XXXXX",
  "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
  "is_enabled": true
}
```

#### Email Channel Example

```json
{
  "type": "email",
  "target": "oncall@example.com",
  "is_enabled": true
}
```

#### Response (`201 Created`)

```json
{
  "channel": {
    "id": "7b889345-d8aa-46d2-a7d5-d0794eb8db14",
    "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
    "type": "webhook",
    "target": "https://hooks.slack.com/services/T00/B00/XXXXX",
    "is_enabled": true,
    "created_at": "2026-11-14T22:15:00Z"
  }
}
```

---

### List Channels

Lists all configured notification channels, optionally filtered by site ID.

- **Method:** `GET`
- **Path:** `/api/notifications`
- **Auth Required:** Yes (`Bearer <token>`)

#### Query Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `site_id` | UUID | No | Filter channels scoped to a specific site |

#### Response (`200 OK`)

```json
{
  "channels": [
    {
      "id": "7b889345-d8aa-46d2-a7d5-d0794eb8db14",
      "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
      "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
      "type": "webhook",
      "target": "https://hooks.slack.com/services/T00/B00/XXXXX",
      "is_enabled": true,
      "created_at": "2026-11-14T22:15:00Z"
    }
  ]
}
```

---

### Update Channel

Modifies the destination target or enabled status of a notification channel.

- **Method:** `PATCH`
- **Path:** `/api/notifications/{id}`
- **Auth Required:** Yes (`Bearer <token>`)
- **Headers:** `Content-Type: application/json`

#### Request Body (All fields optional)

```json
{
  "target": "https://hooks.slack.com/services/T00/B00/NEWKEY",
  "is_enabled": false
}
```

#### Response (`200 OK`)

```json
{
  "channel": {
    "id": "7b889345-d8aa-46d2-a7d5-d0794eb8db14",
    "user_id": "c1f7a0b3-9045-424a-9eb4-850dbe2679dc",
    "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
    "type": "webhook",
    "target": "https://hooks.slack.com/services/T00/B00/NEWKEY",
    "is_enabled": false,
    "created_at": "2026-11-14T22:15:00Z"
  }
}
```

---

### Delete Channel

Permanently deletes a notification channel.

- **Method:** `DELETE`
- **Path:** `/api/notifications/{id}`
- **Auth Required:** Yes (`Bearer <token>`)

#### Response (`200 OK`)

```json
{
  "message": "Notification channel deleted successfully"
}
```

---

### List Notification Events

Retrieves the audit log of sent, failed, or debounce-suppressed alert transitions.

- **Method:** `GET`
- **Path:** `/api/notifications/events`
- **Auth Required:** Yes (`Bearer <token>`)

#### Query Parameters

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `limit` | integer | `50` | Maximum number of events to return |
| `offset` | integer | `0` | Number of events to skip |

#### Response (`200 OK`)

```json
{
  "events": [
    {
      "id": "893c5d71-923f-4e08-8e67-d1a298ff3091",
      "site_id": "e2a4b578-8384-482d-8067-17ebaa619420",
      "channel_id": "7b889345-d8aa-46d2-a7d5-d0794eb8db14",
      "transition": "up_to_down",
      "sent_at": "2026-11-14T22:20:00Z",
      "delivery_status": "sent"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

---

## Quickstart Walkthrough (cURL)

The following end-to-end example demonstrates how to interact with the API using `curl`.

### 1. Verify API Health

```bash
curl -i http://localhost:8080/api/health
```

### 2. Register an Account

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SuperSecretPassword123!",
    "timezone": "UTC"
  }'
```

### 3. Log In & Store Access Token

```bash
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SuperSecretPassword123!"
  }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.token')
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.refresh_token')
echo "Token: $TOKEN"
```

### 4. Create a Monitored Site

```bash
SITE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/sites \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://httpbin.org/status/200",
    "label": "HTTPBin Health",
    "check_interval_sec": 15,
    "timeout_sec": 5,
    "failure_threshold": 2,
    "retry_count": 1,
    "tags": ["demo", "testing"]
  }')

SITE_ID=$(echo $SITE_RESPONSE | jq -r '.site.id')
echo "Created Site ID: $SITE_ID"
```

### 5. Add a Webhook Notification Channel

```bash
curl -X POST http://localhost:8080/api/notifications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"webhook\",
    \"target\": \"https://webhook.site/sample-endpoint\",
    \"site_id\": \"$SITE_ID\",
    \"is_enabled\": true
  }"
```

### 6. Query Dashboard Summary

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/dashboard/summary | jq .
```

### 7. Inspect Site Checks & Latency History

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/sites/$SITE_ID/history?limit=10" | jq .
```

### 8. Rotate Tokens (Refresh)

```bash
curl -X POST http://localhost:8080/api/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}"
```

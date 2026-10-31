# TODO

## PHASE 0: Project Setup
- [X] Create repo (monorepo with `/backend` and `/frontend`, or two repos)
- [X] Set up Go module (`go mod init`)
- [X] Set up React app (Vite recommended)
- [X] Set up Docker + Docker Compose for local dev
- [ ] Set up linting/formatting (`gofmt`/`golangci-lint`, ESLint/Prettier)
- [ ] Set up CI pipeline skeleton (lint + test on push)
- [X] Decide on env/config strategy (`.env` files, `envconfig`/`viper`)

## PHASE 1: Database & Backend Skeleton
- [X] Provision local Postgres (via Docker Compose)
- [X] Set up migrations tool (`golang-migrate`)
- [X] Write initial migrations: `users`, `sites`, `check_results`, `site_status`, `notifications`
- [ ] Set up DB access layer (`sqlc` or `pgx`)
- [X] Write basic health-check endpoint (`GET /api/health`)
- [X] Set up router (`chi`/`gin`/`http`) and middleware chain (logging, recovery, CORS)
- [ ] Set up structured logging (`zap`/`zerolog`/`log`)

## PHASE 2: Authentication
- [X] Implement user registration (hash password with bcrypt/argon2)
- [X] Implement login (issue JWT)
- [X] Implement auth middleware (validate JWT, attach user to request context)
- [ ] Implement refresh token flow
- [ ] Add rate limiting to auth endpoints
- [ ] Write tests for auth flows

## PHASE 3: Sites CRUD
- [ ] `POST /api/sites` — add a site (validate URL, set defaults)
- [ ] `GET /api/sites` — list sites for logged-in user
- [ ] `PATCH /api/sites/:id` — update label/tags/check interval
- [ ] `DELETE /api/sites/:id` — remove/stop monitoring a site
- [ ] Add tagging support (join table or array column)
- [ ] Write tests for sites CRUD + authorization (users can't touch others' sites)

## PHASE 4: Checker Engine
- [ ] Build core HTTP probe function (GET/HEAD, timeout, expected status range)
- [ ] Implement retry-once-on-failure logic
- [ ] Implement consecutive-failure threshold before flipping status
- [ ] Build scheduler (per-site ticker to start)
- [ ] Build worker pool with bounded concurrency
- [ ] Write check results to `check_results` table
- [ ] Update denormalized `site_status` table on each check
- [ ] Handle graceful shutdown (in-flight checks finish before worker exits)
- [ ] Write tests for checker logic (mock HTTP server for various failure modes)

## PHASE 5: Status & History API
- [ ] `GET /api/sites/:id/history` — paginated check results
- [ ] `GET /api/sites/:id/uptime` — uptime % over 24h/7d/30d
- [ ] `GET /api/dashboard/summary` — aggregate up/down/degraded counts
- [ ] Add caching for expensive aggregate queries if needed

## PHASE 6: Notifcations
- [ ] Detect state transitions (up→down, down→up) in checker
- [ ] Build notification dispatch worker
- [ ] Implement email channel (e.g. via SMTP or a provider like SES/Postmark)
- [ ] Add debounce/rate-limiting to avoid notification storms
- [ ] `POST/GET /api/notifications` — manage notification preferences
- [ ] Write tests for transition detection + dispatch

## PHASE 7: Real-time Updates (optional)
- [ ] Decide: WebSocket vs SSE vs polling-only
- [ ] Implement `/api/ws/status` (or SSE equivalent) broadcasting status changes
- [ ] Wire checker/worker to publish updates on state change

## PHASE 8: Frontend Skeleton
- [ ] Set up API client (typed fetch/axios wrapper)
- [ ] Set up React Query for server state
- [ ] Set up routing (React Router)
- [ ] Build Login/Register pages
- [ ] Build auth state handling (store JWT, protected routes)

## PHASE 9: Frontend: Dashboard
- [ ] Build `SiteCard` component (status badge, response time, tags)
- [ ] Build Dashboard page — grid/list of sites
- [ ] Add tag/status filtering
- [ ] Add "Add Site" form/modal
- [ ] Wire up real-time updates (WebSocket/SSE) or polling fallback

## PHASE 10: Frontend: Site Detail
- [ ] Build Site Detail page layout
- [ ] Build uptime % display (24h/7d/30d)
- [ ] Build response-time chart (`recharts`/`visx`)
- [ ] Build incident/history log view
- [ ] Build Edit Site form (label, tags, interval, notifications)

## PHASE 11: Frontend: Settings
- [ ] Build Settings page (account info, notification channels)
- [ ] Wire up notification preference management to backend

## PHASE 12: Frontend: Refinement
- [ ] Add loading/error/empty states across all pages
- [ ] Add form validation (frontend + backend)
- [ ] Add pagination to history views
- [ ] Cross-browser/responsive check on dashboard
- [ ] Add basic accessibility pass (labels, contrast, keyboard nav)

## PHASE 13: Deployment
- [ ] Write Dockerfiles for `api`, `worker`, and frontend
- [ ] Set up production Docker Compose or Kubernetes manifests
- [ ] Set up managed Postgres (or self-hosted with backups)
- [ ] Set up CDN/static hosting for frontend build
- [ ] Set up CI/CD to build + deploy on merge to main
- [ ] Set up domain, TLS certs
- [ ] Smoke test in production
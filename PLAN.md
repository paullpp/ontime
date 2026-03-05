╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ onTime — System Architecture Plan                                                                                  │
│                                                                                                                    │
│ Context                                                                                                            │
│                                                                                                                    │
│ Building a greenfield iOS app ("onTime") that notifies users when to leave for a destination based on real-time    │
│ traffic. The user sets an origin, destination, and desired arrival time. A Go backend polls Google Maps for live   │
│ travel times and sends an APNs push notification when the user should depart.                                      │
│                                                                                                                    │
│ ---                                                                                                                │
│ System Architecture                                                                                                │
│                                                                                                                    │
│ iOS App (Swift/SwiftUI)                                                                                            │
│   └── HTTPS/REST ──► Go Backend Server                                                                             │
│                           ├── HTTP API (chi router)                                                                │
│                           ├── Polling Worker (goroutine pool + Redis sorted set)                                   │
│                           └── Notification Sender (APNs direct, HTTP/2)                                            │
│                                ├── PostgreSQL  (users, trips, devices, notif logs)                                 │
│                                ├── Redis       (poll scheduler + JWT denylist)                                     │
│                                └── Google Maps Routes API                                                          │
│                                         └── APNs ──► iOS Device                                                    │
│                                                                                                                    │
│ Key Data Flow                                                                                                      │
│                                                                                                                    │
│ 1. User creates trip in app → POST /api/v1/trips → Go server persists to PostgreSQL, enqueues in Redis sorted set  │
│ (score = next poll epoch)                                                                                          │
│ 2. Polling worker ticks every 10s, fetches due trip IDs from Redis, calls Google Maps Routes API per trip          │
│ 3. Worker computes should_leave_at = desired_arrival_at - eta_seconds - warning_minutes                            │
│ 4. If now >= should_leave_at and no notification sent yet → APNs push to all user devices                          │
│ 5. iOS app receives push, deep-links to trip detail on tap                                                         │
│                                                                                                                    │
│ ---                                                                                                                │
│ Infrastructure                                                                                                     │
│                                                                                                                    │
│ Go Server Components                                                                                               │
│                                                                                                                    │
│ - HTTP API: chi router with JWT auth middleware + per-IP/per-user rate limiting                                    │
│ - Polling Worker: Redis sorted set trips:poll (score = next poll Unix timestamp). Lua script for atomic            │
│ ZRANGEBYSCORE+ZREM to prevent double-processing across replicas. Bounded goroutine pool (20 workers).              │
│ - Notification Sender: sideshow/apns2 library (APNs HTTP/2, token-based auth with .p8 key — never expires unlike   │
│ certificates)                                                                                                      │
│                                                                                                                    │
│ Databases                                                                                                          │
│                                                                                                                    │
│ - PostgreSQL 16: Users, devices, trips, refresh tokens, notification logs. ACID for status transitions (avoid      │
│ double-send).                                                                                                      │
│ - Redis 7: (1) trips:poll sorted set scheduler; (2) JWT denylist for logout (TTL = remaining token lifetime)       │
│                                                                                                                    │
│ Docker Compose (local dev)                                                                                         │
│                                                                                                                    │
│ services:                                                                                                          │
│   app:      # Go server (port 8080)                                                                                │
│   postgres: # PostgreSQL 16                                                                                        │
│   redis:    # Redis 7                                                                                              │
│   migrate:  # golang-migrate one-shot (runs on startup, depends_on postgres healthcheck)                           │
│                                                                                                                    │
│ Environment Config                                                                                                 │
│                                                                                                                    │
│ Go server reads exclusively from env vars using caarlos0/env. .env gitignored; .env.example committed.             │
│                                                                                                                    │
│ DATABASE_URL, REDIS_URL, JWT_SECRET,                                                                               │
│ APNS_KEY_ID, APNS_TEAM_ID, APNS_KEY_FILE, APNS_BUNDLE_ID,                                                          │
│ GOOGLE_MAPS_API_KEY, ENVIRONMENT                                                                                   │
│                                                                                                                    │
│ ---                                                                                                                │
│ Authentication                                                                                                     │
│                                                                                                                    │
│ Sign in with Apple + JWT                                                                                           │
│                                                                                                                    │
│ 1. iOS triggers ASAuthorizationAppleIDProvider                                                                     │
│ 2. App sends POST /api/v1/auth/apple with identityToken                                                            │
│ 3. Go server verifies token via Apple's JWKS, upserts user in PostgreSQL                                           │
│ 4. Issues access token (JWT, 15-min TTL) + refresh token (opaque, 30-day TTL, stored hashed in PostgreSQL)         │
│ 5. Refresh tokens rotate on use; logout adds access token to Redis denylist                                        │
│                                                                                                                    │
│ Device Push Token                                                                                                  │
│                                                                                                                    │
│ After auth: POST /api/v1/devices registers APNs token. One user can have multiple devices. Tokens marked inactive  │
│ when APNs returns 410 Gone or BadDeviceToken.                                                                      │
│                                                                                                                    │
│ ---                                                                                                                │
│ Google Maps API                                                                                                    │
│                                                                                                                    │
│ Use: Routes API (computeRoutes endpoint) — not legacy Directions API.                                              │
│                                                                                                                    │
│ - routingPreference: TRAFFIC_AWARE_OPTIMAL                                                                         │
│ - travelMode: DRIVE                                                                                                │
│ - departureTime: current time                                                                                      │
│ - X-Goog-FieldMask: routes.duration (basic SKU, ~$0.005/req vs $0.01 for advanced)                                 │
│                                                                                                                    │
│ Two API keys (both restricted in Google Cloud Console):                                                            │
│ - Server key: Routes API only, no app restriction (server-side)                                                    │
│ - iOS key: Maps SDK for iOS only, restricted to app bundle ID (shipped in .xcconfig, gitignored)                   │
│                                                                                                                    │
│ ---                                                                                                                │
│ Core Data Models (PostgreSQL)                                                                                      │
│                                                                                                                    │
│ users         (id UUID, apple_sub TEXT UNIQUE, email TEXT, timestamps)                                             │
│ devices       (id UUID, user_id FK, apns_token TEXT UNIQUE, is_active BOOL, timestamps)                            │
│ refresh_tokens(id UUID, user_id FK, device_id FK, token_hash TEXT, expires_at)                                     │
│ trips         (id UUID, user_id FK,                                                                                │
│                origin_lat/lng/name, destination_lat/lng/name,                                                      │
│                desired_arrival_at TIMESTAMPTZ,                                                                     │
│                warning_minutes INT DEFAULT 0,                                                                      │
│                status TEXT,            -- active|notified|cancelled|expired                                        │
│                latest_eta_seconds INT,                                                                             │
│                next_poll_at TIMESTAMPTZ,                                                                           │
│                notification_sent_at TIMESTAMPTZ,                                                                   │
│                timestamps)                                                                                         │
│ notification_logs (id UUID, trip_id FK, device_id FK, apns_message_id, payload JSONB, status)                      │
│                                                                                                                    │
│ INDEX idx_trips_next_poll ON trips(next_poll_at) WHERE status = 'active'                                           │
│                                                                                                                    │
│ ---                                                                                                                │
│ REST API Endpoints                                                                                                 │
│                                                                                                                    │
│ POST   /api/v1/auth/apple           Sign in with Apple                                                             │
│ POST   /api/v1/auth/refresh         Refresh access token                                                           │
│ DELETE /api/v1/auth/logout          Revoke current device tokens                                                   │
│ DELETE /api/v1/auth/logout-all      Revoke all device tokens                                                       │
│                                                                                                                    │
│ POST   /api/v1/devices              Register APNs token                                                            │
│ DELETE /api/v1/devices/{id}         Unregister device                                                              │
│                                                                                                                    │
│ GET    /api/v1/trips                List active trips                                                              │
│ POST   /api/v1/trips                Create trip                                                                    │
│ GET    /api/v1/trips/{id}           Get trip + current ETA                                                         │
│ PUT    /api/v1/trips/{id}           Update arrival time / warning minutes                                          │
│ DELETE /api/v1/trips/{id}           Cancel trip                                                                    │
│ POST   /api/v1/trips/{id}/activate  Re-activate notified/cancelled trip                                            │
│                                                                                                                    │
│ GET    /health                      Liveness probe                                                                 │
│ GET    /ready                       Readiness probe                                                                │
│                                                                                                                    │
│ ---                                                                                                                │
│ Polling Strategy                                                                                                   │
│                                                                                                                    │
│ Adaptive intervals based on time until arrival:                                                                    │
│                                                                                                                    │
│ ┌────────────────────┬───────────────┐                                                                             │
│ │ Time until arrival │ Poll interval │                                                                             │
│ ├────────────────────┼───────────────┤                                                                             │
│ │ > 6 hours          │ 30 minutes    │                                                                             │
│ ├────────────────────┼───────────────┤                                                                             │
│ │ 2–6 hours          │ 15 minutes    │                                                                             │
│ ├────────────────────┼───────────────┤                                                                             │
│ │ 1–2 hours          │ 10 minutes    │                                                                             │
│ ├────────────────────┼───────────────┤                                                                             │
│ │ 30–60 minutes      │ 5 minutes     │                                                                             │
│ ├────────────────────┼───────────────┤                                                                             │
│ │ 15–30 minutes      │ 3 minutes     │                                                                             │
│ ├────────────────────┼───────────────┤                                                                             │
│ │ < 15 minutes       │ 1 minute      │                                                                             │
│ └────────────────────┴───────────────┘                                                                             │
│                                                                                                                    │
│ Notification hysteresis: Only send if notification_sent_at IS NULL AND ETA has been stable (within ±3 min) for 2   │
│ consecutive polls. Prevents spam from traffic fluctuations.                                                        │
│                                                                                                                    │
│ ---                                                                                                                │
│ iOS App Architecture                                                                                               │
│                                                                                                                    │
│ - Pattern: MVVM + Coordinator (NavigationStack-based router), @Observable macro (iOS 17+ minimum)                  │
│ - Map: MapKit (Map SwiftUI view + MKMapView UIViewRepresentable) — not Google Maps SDK for iOS. Routes calculated  │
│ server-side; app just renders polylines + pins.                                                                    │
│ - Location: WhenInUse authorization only (prefill origin field). No background location needed — server does       │
│ monitoring.                                                                                                        │
│ - Network: URLSession + async/await, APIClient actor with transparent token refresh                                │
│ - Keychain: JWT tokens stored via Keychain wrapper (TokenStore.swift)                                              │
│ - Local fallback: UNCalendarNotificationTrigger scheduled at computed leave time using last known ETA. Cancelled   │
│ when server push arrives.                                                                                          │
│ - Notification taps: Deep link via ontime://trip/{id} custom URL scheme                                            │
│                                                                                                                    │
│ Key Views                                                                                                          │
│                                                                                                                    │
│ AppCoordinator                                                                                                     │
│ ├── AuthView                                                                                                       │
│ ├── TripListView → TripRowView                                                                                     │
│ ├── TripDetailView → MapContainerView, ETABadge, TripInfoCard                                                      │
│ ├── CreateTripView → LocationSearchView (MKLocalSearch), ArrivalTimePicker                                         │
│ └── SettingsView                                                                                                   │
│                                                                                                                    │
│ ---                                                                                                                │
│ Notifications                                                                                                      │
│                                                                                                                    │
│ APNs direct (not Firebase FCM) — iOS-only app, no intermediate hop needed.                                         │
│                                                                                                                    │
│ "Leave Now" alert payload:                                                                                         │
│ {                                                                                                                  │
│   "aps": {                                                                                                         │
│     "alert": { "title": "Time to leave!", "body": "Drive to SFO: 42 min. Depart by 8:48 AM." },                    │
│     "sound": "default",                                                                                            │
│     "interruption-level": "time-sensitive"                                                                         │
│   },                                                                                                               │
│   "trip_id": "...",                                                                                                │
│   "type": "leave_now"                                                                                              │
│ }                                                                                                                  │
│ Requires com.apple.developer.usernotifications.time-sensitive entitlement (breaks through Focus modes).            │
│                                                                                                                    │
│ Silent ETA sync (content-available: 1) sent periodically to keep TripListView fresh when app is backgrounded.      │
│                                                                                                                    │
│ ---                                                                                                                │
│ Directory Structure                                                                                                │
│                                                                                                                    │
│ Go Server (server/)                                                                                                │
│                                                                                                                    │
│ cmd/server/main.go                                                                                                 │
│ internal/                                                                                                          │
│   api/router.go                                                                                                    │
│   api/middleware/auth.go, ratelimit.go                                                                             │
│   api/handlers/auth.go, devices.go, trips.go                                                                       │
│   auth/apple.go, jwt.go                                                                                            │
│   config/config.go                                                                                                 │
│   db/db.go, queries/*.sql.go                                                                                       │
│   models/trip.go, user.go, device.go                                                                               │
│   maps/routes.go                                                                                                   │
│   notifications/apns.go                                                                                            │
│   worker/supervisor.go, processor.go                                                                               │
│ migrations/000001_init_schema.up.sql                                                                               │
│ docker/Dockerfile                                                                                                  │
│ docker-compose.yml                                                                                                 │
│ .env.example                                                                                                       │
│ Makefile                                                                                                           │
│                                                                                                                    │
│ Key Go deps: go-chi/chi, jackc/pgx/v5, redis/go-redis/v9, sideshow/apns2, golang-jwt/jwt/v5, caarlos0/env,         │
│ golang.org/x/time/rate, golang-migrate/migrate                                                                     │
│                                                                                                                    │
│ iOS App (onTime/)                                                                                                  │
│                                                                                                                    │
│ onTime.xcodeproj/                                                                                                  │
│ onTime/                                                                                                            │
│   App/onTimeApp.swift, AppCoordinator.swift                                                                        │
│   Features/Auth/, TripList/, TripDetail/, CreateTrip/, Settings/                                                   │
│   Core/Network/APIClient.swift, TokenStore.swift, Endpoints.swift                                                  │
│   Core/Models/Trip.swift, User.swift, Device.swift                                                                 │
│   Core/Notifications/NotificationManager.swift, LocalFallback.swift                                                │
│   Core/Location/LocationManager.swift                                                                              │
│   Supporting/Config.xcconfig, onTime.entitlements                                                                  │
│                                                                                                                    │
│ ---                                                                                                                │
│ Implementation Phases                                                                                              │
│                                                                                                                    │
│ 1. Foundation: Go scaffold, DB schema, Docker Compose, health endpoints, Apple Sign In, JWT middleware             │
│ 2. Core API: Trip CRUD, device registration, Google Maps client, Redis scheduler                                   │
│ 3. Worker + Notifications: Polling supervisor, adaptive intervals, APNs sender, hysteresis logic                   │
│ 4. iOS App: Auth flow, APNs registration, map + trip creation UI, trip list/detail, deep links, local fallback     │
│ 5. Polish: Rate limiting, error handling/retry, silent push sync, time-sensitive entitlement, E2E testing          │
│                                                                                                                    │
│ ---                                                                                                                │
│ Verification                                                                                                       │
│                                                                                                                    │
│ - docker compose up starts all services; GET /ready returns 200 when DB + Redis connected                          │
│ - Create a trip via POST /api/v1/trips with desired_arrival_at 5 minutes from now                                  │
│ - Observe worker logs showing Google Maps API calls and countdown to notification                                  │
│ - Verify APNs delivery appears in iOS Simulator (or device) within the expected window                             │
│ - Confirm notification_logs row inserted in PostgreSQL after send                                                  │
│ - Test edge cases: APNs invalid token (device marked inactive), traffic spike (ETA hysteresis), app in background  │
│ (local fallback fires)                                                                                             │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────

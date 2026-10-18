# Comprehensive Fix Plan: Business (B1-B7), Delete Blast Email (B8), Technical (T1-T12), & Cloudinary Image Compression

## Goal
Resolve all business vulnerabilities (B1-B7), remove the legacy blast-email system (B8), remediate all technical debt and security risks (T1-T12), implement Cloudinary automated image compression transformations on URL generation, and establish an automated test suite.

---

## Current Context & Assumptions
- **Language/Framework**: Go 1.25.4, Gin Gonic, GORM with PostgreSQL driver, Redis (Upstash) via `go-redis/v9`.
- **Branch**: `feature/manual-payment-system`.
- **Existing Deficiencies**:
  - `B1 / T1`: Public `/api/seed-admin` endpoint with hardcoded credentials `admin` / `ringkaibinar`.
  - `B2`: Orders have `expires_at` (24h) but no background cleaner/sweeper, blocking total quota indefinitely.
  - `B3`: Order IDs generated via `SMILE-%d` (Unix timestamp in seconds), causing collisions on concurrent checkouts.
  - `B4`: Total ticket quota (600) checked without DB row lock or atomic transaction boundary.
  - `B5`: Voucher quota verified during checkout but only incremented during payment verification without concurrency lock.
  - `B6`: Revenue stats (`SUM(total_amount)`) mix ticket price and random 3-digit `unique_code`, voucher discounts aren't logged per order.
  - `B7`: `scanned_tickets` counts all scanned tickets without verifying transaction settlement status.
  - `B8`: Blast email functionality is requested to be completely deleted.
  - `T2`: Error checking relies on strict raw string comparison `err.Error() == "PENDING_TRANSACTION_EXISTS"`.
  - `T3`: JWT claims unchecked for signing method & panics on missing/invalid `role` type assertion.
  - `T4`: Upload payment proof accepts any MIME/file size without restriction and returns HTTP 500 on validation errors.
  - `T5 / T6`: In-memory worker queue is volatile, unbuffered beyond 200, lacks retry and durable storage.
  - `T7`: 0 unit/integration tests exist (`*_test.go`).
  - `T8`: `CountActiveTickets()` error ignored with `_`.
  - `T9`: DB `AutoMigrate` runs directly on main startup without separation or migration locks.
  - `T10`: `.env.example` missing key environment variables (`REDIS_URL`, `CLOUDINARY_URL`, etc.).
  - `T11`: `/api/tickets/track` returns sensitive buyer demographic profile fields to unauthenticated requesters.
  - `T12`: Checkout and upload endpoints lack rate-limiting.
  - **Cloudinary Compression**: Cloudinary URLs for payment proof uploads should automatically inject transformations (`f_auto,q_auto,w_1000,c_limit`) to save bandwidth and speed up mobile admin review.

---

## Architecture & Proposed Approach
1. **Domain Errors & Concurrency Control**: Define structured package-level typed errors in `pkg/errors` or `internal/service`. Enforce PostgreSQL pessimistic locking (`SELECT ... FOR UPDATE`) or atomic SQL queries for ticket quota count and voucher quota reservations.
2. **Order ID & Security**: Replace seconds-based Unix ID with high-entropy short alphanumeric ID (NanoID or UUIDv7-based timestamp prefix + random hex `SMILE-YYYYMMDD-<uuid/random>`). Secure JWT parsing with explicit HMAC method validation and safe claim extraction. Eliminate `/api/seed-admin`.
3. **Sweeper & Image Transformation**: Introduce an asynchronous sweeper routine running every N minutes to transition stale `pending` transactions (`expires_at < NOW()`) into `expired`. Enhance Cloudinary wrapper to generate and store auto-optimized transformation URLs.

---

## Step-by-Step Implementation Tasks

### Phase 1: Removal of Legacy Blast Email (B8)
- **Task 1.1**: Remove Blast Email from Router & Handler
  - Files:
    - `internal/router/router.go` (remove `admin.POST("/transactions/blast-email", adminHandler.BlastEmail)`)
    - `internal/handler/admin_handler.go` (remove `BlastEmail` method)
  - Verification: `go build ./...` passes.

- **Task 1.2**: Remove Blast Email from Service & Repository
  - Files:
    - `internal/service/transaction_service.go` (remove `SendBlastEmailToSettledTransactions` from interface and struct)
    - `internal/repository/transaction_repo.go` (remove `GetSettledEmailsForBlast` from interface and struct)
  - Verification: `go build ./...` passes.

- **Task 1.3**: Remove Blast Email from Worker & Utils
  - Files:
    - `internal/worker/email_worker.go` (remove `TaskBlast`, `BlastData`, and switch case)
    - `pkg/utils/email.go` (remove `SendBlastEmail` and `BlastData` struct)
  - Verification: `go test ./...` and `go build ./...` succeed without unused identifier errors.

---

### Phase 2: Security & Authentication Hardening (B1, T1, T3)
- **Task 2.1**: Remove Public Seed Admin & Hardcoded Password (B1, T1)
  - Files:
    - `internal/router/router.go` (delete `/api/seed-admin` route)
    - `internal/handler/user_handler.go` (delete `SeedAdmin` handler method)
    - `internal/service/user_service.go` (delete `SeedAdmin` method and interface signature)
    - `cmd/seed/main.go` (optional standalone CLI tool if admin seeding is required via env var, or check if admin exists on boot via secure ENV vars `ADMIN_INIT_USERNAME` & `ADMIN_INIT_PASSWORD` with bcrypt hash).
  - Verification: Run `curl -X POST http://localhost:8080/api/seed-admin` returning 404 Not Found.

- **Task 2.2**: Secure JWT Validation & Safe Claim Parsing (T3)
  - Files:
    - `internal/middleware/auth.go`
  - Implementation:
    - Validate `token.Method.(*jwt.SigningMethodHMAC)` explicitly.
    - Check token expiry explicitly.
    - Extract claims safely using type checks (`val, ok := claims["role"].(string)`), returning 401 if missing instead of panicking.
  - Verification: Send requests with malformed tokens, tokens signed with `none` algorithm, and tokens without `role` claim. Ensure proper 401/403 responses without panics.

---

### Phase 3: Transaction Concurrency, Quota & Order ID (B3, B4, B5, T2)
- **Task 3.1**: Standardize Custom Errors (T2)
  - File: `internal/service/errors.go` (or `pkg/apperror/errors.go`)
  - Define `ErrPendingTransactionExists = errors.New("pending transaction exists")`, `ErrTicketQuotaExceeded`, `ErrVoucherInvalidOrExhausted`, etc.
  - Update `internal/handler/transaction_handler.go` to check using `errors.Is(err, service.ErrPendingTransactionExists)` instead of comparing raw error strings.

- **Task 3.2**: Collision-Resistant Order ID (B3)
  - File: `internal/service/transaction_service.go`
  - Implementation: Format order ID using timestamp millisecond/nanosecond + cryptographically random hex, e.g., `fmt.Sprintf("SMILE-%s-%s", time.Now().Format("0601021504"), strings.ToUpper(uuid.New().String()[:6]))`.
  - Verification: Write concurrency unit test generating 1,000 Order IDs in parallel; assert 0 duplicates.

- **Task 3.3**: Atomic Ticket Quota Check & Reservation (B4, T8)
  - Files:
    - `internal/repository/transaction_repo.go`
    - `internal/service/transaction_service.go`
  - Implementation:
    - Wrap checkout check and creation inside an explicit DB transaction with row-level locking or check ticket count inside `tx.Raw(...)` with `FOR UPDATE` or conditional insert.
    - Fix T8 by properly handling the returned `error` from `CountActiveTickets()` instead of discarding with `_`.

- **Task 3.4**: Atomic Voucher Quota Reservation (B5)
  - Files:
    - `internal/repository/voucher_repo.go`
    - `internal/service/transaction_service.go`
    - `internal/repository/transaction_repo.go`
  - Implementation:
    - Reserve voucher quota at checkout time atomically via `UPDATE vouchers SET usage_count = usage_count + ? WHERE id = ? AND (quota - usage_count) >= ?`.
    - If transaction expires or is cancelled, decrement `usage_count` (or maintain a dedicated `reserved_count` column).

---

### Phase 4: Order Expiry Sweeper & Financial Integrity (B2, B6, B7)
- **Task 4.1**: Stale Order Sweeper Worker (B2)
  - Files:
    - `internal/repository/transaction_repo.go` (add `ExpireStaleTransactions() (int64, error)`)
    - `internal/worker/sweeper_worker.go` (new background routine running every 5-10 minutes with `time.Ticker`, updating `status = 'expired'` where `status = 'pending' AND expires_at < NOW()`)
    - `cmd/api/main.go` (launch `go worker.StartOrderSweeper(ctx, trxRepo)`)
  - Verification: Insert pending transaction with past `expires_at`, trigger sweeper, confirm status updates to `expired` and released tickets don't block new checkouts.

- **Task 4.2**: Financial Disaggregation & Revenue Calculation (B6)
  - Files:
    - `internal/model/models.go` (add `BaseAmount float64`, `DiscountAmount float64` to `Transaction`)
    - `internal/service/transaction_service.go` (save `base_amount` and `discount_amount` explicitly during checkout)
    - `internal/repository/transaction_repo.go` (`GetDashboardStats` calculates actual ticket revenue via `SUM(base_amount - discount_amount)` rather than including random `unique_code`).

- **Task 4.3**: Fix Scanned Ticket Metric Accuracy (B7)
  - Files:
    - `internal/repository/transaction_repo.go`
    - `internal/repository/ticket_repo.go`
  - Implementation:
    - Ensure `scannedTickets` count joins with `transactions` table to filter only `transactions.status = 'settlement'`.

---

### Phase 5: Technical Debt, Privacy & Resilience (T4, T5, T6, T9, T10, T11, T12)
- **Task 5.1**: Cloudinary Image Compression & Upload Validation (T4 + User Request)
  - Files:
    - `internal/handler/transaction_handler.go` (validate max size e.g. 5MB, inspect MIME type header `image/jpeg, image/png, image/webp`, return HTTP 400 Bad Request on failure instead of 500)
    - `pkg/cloudinary/client.go` (add URL transformation params: `f_auto,q_auto,w_1000,c_limit` or append Cloudinary transformation path so image delivery is automatically compressed)
  - Verification: Upload sample image; verify returned URL contains optimization parameters.

- **Task 5.2**: Privacy Protection on Public Tracking Endpoint (T11)
  - Files:
    - `internal/model/models.go` or `internal/handler/ticket_handler.go`
    - Create a sanitized DTO `TrackTicketResponse` that omits `ProfileAge`, `ProfileCity`, `ProfileEducation`, `ProfileJob`, `InterestReasons`, `SustainabilitySteps`, etc.
  - Verification: Call `GET /api/tickets/track` with order_id and email; verify demographic survey fields are excluded.

- **Task 5.3**: Worker Queue Resilience & Graceful Shutdown (T5, T6)
  - Files:
    - `internal/worker/email_worker.go`
    - `cmd/api/main.go`
  - Implementation:
    - Support graceful shutdown using `context.Context` to flush in-flight jobs.
    - If Redis is available, persist failed or pending jobs into a Redis queue/list; fallback to in-memory channel with logged warnings if Redis is down.

- **Task 5.4**: Rate Limiter Middleware for Public Endpoints (T12)
  - Files:
    - `internal/middleware/rate_limit.go` (IP-based token bucket using Redis if connected, or memory cache fallback)
    - `internal/router/router.go` (apply to `/api/checkout` and `/api/transactions/:id/upload-proof`)

- **Task 5.5**: Clean Database Boot & Documentation (T9, T10)
  - Files:
    - `internal/config/database.go` (wrap AutoMigrate with environment check `APP_ENV != "production"` or dedicated migration flag)
    - `.env.example` (add all required & optional env vars: `PORT`, `DB_URL`, `JWT_SECRET`, `REDIS_URL`, `CLOUDINARY_URL`, `GAS_URL`, `GIN_MODE`, `APP_ENV`)

---

### Phase 6: Automated Testing Suite (T7)
- **Task 6.1**: Setup Test Suite & Mocks / Test DB
  - Files:
    - `internal/service/transaction_service_test.go`
    - `internal/handler/ticket_handler_test.go`
    - `internal/middleware/auth_test.go`
  - Implementation:
    - Unit tests for Checkout calculations (discount, unique code, total amount).
    - Unit tests for JWT middleware (valid token, expired token, wrong role, missing role).
    - Unit tests for Voucher validation & quota.
    - Verification: `go test -v -cover ./...` passes.

---

## Risks, Tradeoffs, and Open Questions
1. **Database Schema Changes**: Adding `BaseAmount` and `DiscountAmount` to `Transaction` model requires running migration on existing databases without breaking existing rows (defaults to 0 or total_amount - unique_code).
2. **Voucher Quota Rollback**: If a voucher is reserved at checkout and the order expires, returning the quota to the voucher pool requires reliable status transitions.
3. **Cloudinary Transformation URL**: Cloudinary supports URL-based transformations (`/upload/f_auto,q_auto/...`) without re-uploading original files. We can inject this either at upload time or on response serialization.

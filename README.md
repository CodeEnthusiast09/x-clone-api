# x-clone-api

Go backend for the X (Twitter) clone mobile app. Companion to `../x-clone-expo` (not built yet).

A learning rebuild of [burakorkmez/x-clone-rn](https://github.com/burakorkmez/x-clone-rn) — same UX, new tech, own folder conventions.

---

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.26 |
| HTTP framework | Gin |
| ORM | GORM |
| Database | PostgreSQL |
| Auth | Clerk (JWT verification via `clerk-sdk-go/v2`) |
| Webhook verification | Svix (Clerk webhook signatures) |
| Image upload | Cloudinary (presigned, direct-from-mobile uploads) |
| Future: rate limit + bots | Arcjet |
| Future: real-time chat | `gorilla/websocket` |
| Env loading | `joho/godotenv` |

---

## Quick start

1. Postgres running; database from `DATABASE_URL` exists.
2. `cp .env.example .env`, fill in real values from Clerk + Cloudinary dashboards.
3. `go mod tidy`
4. `go run ./cmd`
5. Verify: `curl http://localhost:8080/health` → `{"success":true,"message":"ok",...}`

---

## Environment variables

See `.env.example`. All of these are required at boot (`config.Load()` uses `mustGet`):

| Key | Notes |
|---|---|
| `PORT` | Defaults to `8080` if unset |
| `ENV` | `development` or `production`; production switches Gin to release mode |
| `DATABASE_URL` | Standard Postgres DSN |
| `CLERK_SECRET_KEY` | From Clerk dashboard → API keys |
| `CLERK_PUBLISHABLE_KEY` | Stored only; mobile uses it |
| `CLERK_WEBHOOK_SECRET` | From Clerk dashboard → Webhooks endpoint |
| `CLOUDINARY_CLOUD_NAME` / `_API_KEY` / `_API_SECRET` | From Cloudinary dashboard |
| `CLOUDINARY_UPLOAD_PRESET` | Must equal the preset name created in the Cloudinary dashboard. Default: `x_clone_posts` (the preset is reused for every signed upload — name kept for backwards compat) |
| `POST_IMAGE_MAX_BYTES` | Post-image cap, enforced server-side via signed params. Default: `5242880` (5 MB) |
| `BANNER_IMAGE_MAX_BYTES` | User-banner cap, same enforcement mechanism. Default: `5242880` (5 MB, matching X) |
| `ARCJET_KEY` / `ARCJET_ENV` | Read at boot; not wired into middleware yet (Phase 4) |

---

## Cloudinary upload preset (dashboard config)

A signed upload preset named `x_clone_posts` must exist in the Cloudinary dashboard. Settings:

- **Signing mode:** Signed
- **Asset folder:** `x_clone/posts`
- **Overwrite assets with same public ID:** Off
- **Generated public ID:** Auto-generate an unguessable value (we override this per-upload to a user-namespaced path)
- **Generated display name:** Use last segment of public ID (don't leak original filenames)
- **Allowed formats:** `jpg,png,webp`

File size (5 MB) and image dimensions are enforced by the backend via signed `max_bytes` rather than the preset (the current Cloudinary console doesn't expose these on the per-preset GUI).

---

## Folder structure

```
cmd/
└── main.go                  entry point — config, DB, Cloudinary, router, HTTP server
internal/
├── config/                  env loading + Config struct (mustGet for required vars)
├── db/                      GORM Postgres connect + AutoMigrate at startup
├── middleware/              Clerk JWT auth (Arcjet + CORS will land here later)
├── models/                  GORM models — User, Post, Comment (UUID PKs via gen_random_uuid)
├── router/                  Gin engine + route registration coordinator
├── common/                  response envelope helpers — Success / Error / Paginated
├── cloudinary/              signing, destroy, URL parsing — pure stdlib SHA1, no SDK
├── users/                   user reads + /me + /auth/sync
├── posts/                   post CRUD + likes/unlikes
├── comments/                comment reads (writes come in Phase 3c)
├── uploadsignatures/        POST /api/upload-signatures
└── webhooks/                POST /api/webhooks/clerk (Svix-verified)
scripts/
└── test-phase-3b.sh         e2e test runner — mints JWT via Clerk Backend API, hits all endpoints
```

Each feature folder has the same three-file shape: `service.go` (DB + business logic), `handler.go` (HTTP layer), `routes.go` (registration).

---

## API endpoints

### Public (no auth)

| Method | Path | Description |
|---|---|---|
| `GET`  | `/health` | uptime + env |
| `GET`  | `/api/users/:username` | user profile |
| `GET`  | `/api/posts` | paginated feed (`?page=&limit=`) |
| `GET`  | `/api/posts/:postId` | single post (with comments) |
| `GET`  | `/api/users/:username/posts` | a user's posts (paginated) |
| `GET`  | `/api/comments/post/:postId` | comments for a post |
| `POST` | `/api/webhooks/clerk` | Clerk user lifecycle (Svix-verified) |

### Authenticated (Clerk JWT in `Authorization: Bearer ...`)

| Method | Path | Description |
|---|---|---|
| `GET`    | `/api/me` | the authed user's row in our DB |
| `PATCH`  | `/api/me` | partial profile update (firstName, lastName, bio, location, bannerImage) |
| `POST`   | `/api/auth/sync` | fallback when the webhook hasn't created the user yet; idempotent |
| `POST`   | `/api/upload-signatures/posts` | signed Cloudinary upload params for a post image (user-scoped public_id) |
| `POST`   | `/api/upload-signatures/banners` | signed Cloudinary upload params for a banner image (user-scoped public_id) |
| `POST`   | `/api/posts` | create post (text and/or image) |
| `DELETE` | `/api/posts/:postId` | delete (single-query ownership + Cloudinary destroy) |
| `POST`   | `/api/posts/:postId/likes` | like (idempotent, 204) |
| `DELETE` | `/api/posts/:postId/likes` | unlike (idempotent, 204) |
| `POST`   | `/api/posts/:postId/comments` | create a comment (text only, <=280 chars) |
| `DELETE` | `/api/comments/:commentId` | delete a comment (single-query ownership) |

### Response envelope

All endpoints return one of:

```json
{ "success": true,  "message": "...", "data": <any> }
{ "success": false, "message": "..." }
```

Paginated reads add `meta`:

```json
{ "success": true, "message": "...", "data": [...],
  "meta": { "total": 0, "page": 1, "limit": 20, "totalPages": 0 } }
```

---

## Security notes worth remembering

- **Cloudinary upload public_ids are owner-scoped**: posts land under `x_clone/posts/users/<clerkID>/<uuid>`, banners under `x_clone/banners/users/<clerkID>/<uuid>`. The public_id is included in the signed params, so the mobile client cannot upload to a different path without invalidating the signature.
- **Post creation rejects foreign image URLs**: if you `POST /api/posts` with `image: "<URL>"` whose extracted public_id doesn't start with your posts prefix, you get **400**. Prevents an IDOR where one user references another user's Cloudinary asset on their own post and then deletes it.
- **`PATCH /api/me` applies the same check to `bannerImage`**: the URL must extract a public_id starting with `x_clone/banners/users/<clerkID>/`. Same defense pattern, applied at write time.
- **Single-query ownership for DELETE**: `DELETE FROM posts WHERE id=? AND user_id=?`. Returns 404 for both "doesn't exist" and "exists but not yours" — avoids leaking existence via 403-vs-404.
- **Webhook signatures verified** via Svix using `CLERK_WEBHOOK_SECRET` — unsigned requests get 401 before any DB work.

---

## Gin routing gotchas worth remembering

Gin uses a strict radix tree. Two rules:

1. **A literal child and a `:param` child can coexist under the same node IF the literal has children.** Panic only when both are terminal handlers at the same depth — e.g. `/users/me` next to `/users/:username` (both end at depth 2) explodes. `/posts/:postId` next to `/posts/user/:username` is fine because `user` is a pit stop with `:username` deeper.
2. **Param names must be consistent across the subtree.** If you use `:postId` once under `/posts/`, every route through that subtree must use `:postId` — not `:id`, not `:post_id`.

---

## Conventions

- **Strict everywhere**: explicit error handling, no panics in normal flow (`log.Fatalf` only at startup).
- **Feature folders own their routes**: `feature/routes.go::Register` and `RegisterProtected`.
- **Cross-feature route mounting**: handler logic stays in the feature that owns the data, URL gets mounted under the appropriate prefix. Example: `posts.RegisterUnderUsers(api, db)` mounts `GET /users/:username/posts` even though the handler is in `internal/posts/`.
- **Service methods take `clerkID` directly** and resolve the internal user UUID internally. Handlers stay thin.
- **All env vars read through `config.Load()`** — no `os.Getenv` calls in handlers or services.

---

## Testing

There's no Go test suite yet. End-to-end testing is a bash script at `scripts/test-phase-3b.sh`. It:

1. Reads `CLERK_SECRET_KEY` from `.env`
2. Queries Clerk Backend API for the test user's active session
3. Mints a fresh JWT for that session
4. Hits all the Phase 3b endpoints (positive + negative paths + IDOR check) using that JWT

Because mint + run happens locally (sub-second), Clerk's default 60-second JWT is plenty.

To run:

```bash
chmod +x scripts/test-phase-3b.sh
./scripts/test-phase-3b.sh
```

To test against a different Clerk user without editing the script:

```bash
CLERK_USER_ID=user_xxx ./scripts/test-phase-3b.sh
```

If there's no active session, open Clerk dashboard → click the test user → **Actions** → **Impersonate user** once to spawn one.

---

## Roadmap (phase = commit checkpoint)

- ✅ (1) backend foundation — `9822e4b`
- ✅ (2) models + read endpoints — `c989c33`
- ✅ (3a) Clerk auth + webhooks + `/me` + `/sync` — `f04656f`
- ✅ refactor: nest user-scoped post reads — `2a12164`
- ✅ (3b) Cloudinary + post writes + IDOR-safe owner-scoped paths — `c97a910`
- ⬜ (3c) Comment writes + profile update + follow toggle
- ⬜ (4) Arcjet middleware (rate limit + bot detection)
- ⬜ (5) WebSocket chat (new feature vs the original — `gorilla/websocket`)
- ⬜ (6) Mobile scaffold (separate repo: `x-clone-expo` — Expo SDK 54+, NativeWind, Yarn)
- ⬜ (7) Port screens: auth → home → notifications → profile → search → messages

---

## Useful pointers

- Original repo (reference, not modified): `../x-clone-rn`
- Sibling project this borrows patterns from: `../../proctura-backend`
- Cloudinary dashboard: https://console.cloudinary.com
- Clerk dashboard: https://dashboard.clerk.com

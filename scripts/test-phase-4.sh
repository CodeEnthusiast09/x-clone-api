#!/bin/bash
#
# Phase 4 e2e test runner -- Arcjet middleware.
# Verifies:
#   * /health and /api/webhooks/clerk are NOT rate-limited
#   * Bot detection is DryRun in development (request still goes through)
#   * Each tier (public 60/min, auth 30/min, write 20/min) returns 429 once
#     its TokenBucket is drained, and RateLimit-* headers are present
#
# Usage:
#   chmod +x scripts/test-phase-4.sh
#   ./scripts/test-phase-4.sh
#
# To skip the burst tests (they pollute the Arcjet dashboard and the DB,
# and the per-IP buckets need ~1 minute to refill before you can re-run):
#   SKIP_BURST=1 ./scripts/test-phase-4.sh
#
# Requires:
#   - The x-clone-api server running on :8080 (ENV=development)
#   - psql installed (used to clean up burst-test posts at the end)
#   - .env at the repo root with CLERK_SECRET_KEY and DATABASE_URL filled in
#   - At least one active Clerk session for the user under test
#     (open Clerk dashboard, "Actions > Impersonate user" once if needed)

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
BASE_URL="http://localhost:8080"

MY_CLERK_ID="${CLERK_USER_ID:-user_3EdzrQVevZevoSIOmY5ADRKukMx}"
SKIP_BURST="${SKIP_BURST:-0}"

# Marker used in burst-write content so we can clean up only our test rows.
WRITE_MARKER="phase-4-write-burst-$(date +%s)"

# --- 0. Load secret + DB URL, mint a fresh JWT ---
SECRET=$(grep '^CLERK_SECRET_KEY=' "$ENV_FILE" | cut -d= -f2-)
DB_URL=$(grep '^DATABASE_URL=' "$ENV_FILE" | cut -d= -f2-)
if [ -z "$SECRET" ] || [ -z "$DB_URL" ]; then
    echo "ERROR: could not read CLERK_SECRET_KEY or DATABASE_URL from $ENV_FILE"
    exit 1
fi

echo "Looking up active session for $MY_CLERK_ID ..."
SESSIONS_RESP=$(curl -s \
    -H "Authorization: Bearer $SECRET" \
    -H "Content-Type: application/json" \
    "https://api.clerk.com/v1/sessions?user_id=$MY_CLERK_ID&status=active")
SESSION_ID=$(echo "$SESSIONS_RESP" | grep -oP '"id":"\Ksess_[^"]+' | head -1)

if [ -z "$SESSION_ID" ]; then
    echo "ERROR: no active session found for user $MY_CLERK_ID"
    echo "Open the Clerk dashboard, click Actions > 'Impersonate user' on the test user once."
    echo "API response: $SESSIONS_RESP"
    exit 1
fi

echo "Minting JWT via Clerk Backend API ..."
TOKEN_RESP=$(curl -s -X POST \
    -H "Authorization: Bearer $SECRET" \
    -H "Content-Type: application/json" \
    "https://api.clerk.com/v1/sessions/$SESSION_ID/tokens")
JWT=$(echo "$TOKEN_RESP" | grep -oP '"jwt"\s*:\s*"\K[^"]+')

if [ -z "$JWT" ]; then
    echo "ERROR: minting JWT failed. Response: $TOKEN_RESP"
    exit 1
fi
echo "Minted JWT (${#JWT} chars). Running tests ..."
echo ""

AUTH="Authorization: Bearer $JWT"
CT="Content-Type: application/json"

# =================================================================
#   1. UNTHROTTLED ROUTES
# =================================================================

echo "============================================================"
echo "Test 1: GET /health x 100 -- expect zero 429s (no Arcjet)"
echo "============================================================"
fail=0
for i in $(seq 1 100); do
    code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
    if [ "$code" != "200" ]; then
        fail=$((fail+1))
        echo "  iteration $i: got $code (expected 200)"
    fi
done
if [ "$fail" -eq 0 ]; then
    echo ">>> all 100 /health requests returned 200 (unthrottled, as designed)"
else
    echo ">>> FAIL: $fail of 100 /health requests did not return 200"
fi
echo ""

echo "============================================================"
echo "Test 2: POST /api/webhooks/clerk (no signature) -- expect 4xx but NOT 429"
echo "============================================================"
# Webhooks must reach the Svix signature check, not be filtered by Arcjet.
# We send an empty body, so we expect 400/401, never 429.
code=$(curl -s -o /tmp/webhook-out -w "%{http_code}" -X POST -H "$CT" \
    -d '{}' "$BASE_URL/api/webhooks/clerk")
if [ "$code" = "429" ]; then
    echo ">>> FAIL: webhook hit Arcjet rate limit ($code) -- it should bypass Arcjet"
else
    echo ">>> webhook returned $code (signature check reached, not Arcjet 429)"
fi
echo ""

# =================================================================
#   2. BOT DETECTION (DryRun in dev)
# =================================================================

echo "============================================================"
echo "Test 3: GET /api/posts with bot-ish UA -- expect 200 in dev (DryRun)"
echo "============================================================"
# Arcjet flags 'curl' / generic SDK UAs as automated. In ENV != production,
# DetectBot runs in DryRun: the decision is logged on the Arcjet dashboard
# but the request still goes through.
BOT_RESP=$(curl -s -i -A "go-http-client/1.1" "$BASE_URL/api/posts?page=1&limit=1")
echo "$BOT_RESP" | head -8
status=$(echo "$BOT_RESP" | head -1 | grep -oP '\d{3}')
if [ "$status" = "200" ]; then
    echo ">>> bot-UA request succeeded ($status) -- check Arcjet dashboard for the dry-run log"
else
    echo ">>> got $status (expected 200 in DryRun dev mode -- either prod mode or buckets drained)"
fi
echo ""

echo "============================================================"
echo "Test 4: RateLimit-* headers present on a normal authed request"
echo "============================================================"
HEADERS=$(curl -s -D - -o /dev/null -H "$AUTH" "$BASE_URL/api/me")
echo "$HEADERS" | grep -i -E '^(ratelimit|http/)' || echo "(no RateLimit-* headers found)"
echo ""

if [ "$SKIP_BURST" = "1" ]; then
    echo "============================================================"
    echo "SKIP_BURST=1 -- skipping burst tests (5/6/7)."
    echo "============================================================"
    exit 0
fi

# =================================================================
#   3. TIER BURST TESTS
# =================================================================
# Each tier's TokenBucket is per-IP and refills at rpm tokens/min.
# We send slightly more requests than the limit and expect the tail
# to start returning 429 once the bucket is empty.
#
# Note: these tests deplete the buckets for ~1 minute. Re-running the
# script before the buckets refill will show 429s from the start.

echo "============================================================"
echo "Test 5: BURST public-read tier -- GET /api/posts x 80 (limit 60/min)"
echo "============================================================"
count_200=0
count_429=0
count_other=0
for i in $(seq 1 80); do
    code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/posts?page=1&limit=1")
    case "$code" in
        200) count_200=$((count_200+1)) ;;
        429) count_429=$((count_429+1)) ;;
        *)   count_other=$((count_other+1)) ;;
    esac
done
echo ">>> 200s=$count_200  429s=$count_429  other=$count_other"
echo "    expected: ~60 200s then ~20 429s (60/min bucket)"
if [ "$count_429" -gt 0 ]; then
    echo ">>> public-read tier IS rate-limiting"
else
    echo ">>> FAIL: no 429s -- public tier may not be wired"
fi
echo ""

echo "============================================================"
echo "Test 6: BURST authed-read tier -- GET /api/me x 40 (limit 30/min)"
echo "============================================================"
count_200=0
count_429=0
count_other=0
for i in $(seq 1 40); do
    code=$(curl -s -o /dev/null -w "%{http_code}" -H "$AUTH" "$BASE_URL/api/me")
    case "$code" in
        200) count_200=$((count_200+1)) ;;
        429) count_429=$((count_429+1)) ;;
        *)   count_other=$((count_other+1)) ;;
    esac
done
echo ">>> 200s=$count_200  429s=$count_429  other=$count_other"
echo "    expected: ~30 200s then ~10 429s (30/min bucket)"
if [ "$count_429" -gt 0 ]; then
    echo ">>> authed-read tier IS rate-limiting"
else
    echo ">>> FAIL: no 429s -- authed-read tier may not be wired"
fi
echo ""

echo "============================================================"
echo "Test 7: BURST write tier -- POST /api/posts x 30 (limit 20/min)"
echo "============================================================"
count_201=0
count_429=0
count_other=0
for i in $(seq 1 30); do
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "$AUTH" -H "$CT" \
        -d "{\"content\":\"$WRITE_MARKER $i\"}" "$BASE_URL/api/posts")
    case "$code" in
        201) count_201=$((count_201+1)) ;;
        429) count_429=$((count_429+1)) ;;
        *)   count_other=$((count_other+1)) ;;
    esac
done
echo ">>> 201s=$count_201  429s=$count_429  other=$count_other"
echo "    expected: ~20 201s then ~10 429s (20/min bucket)"
if [ "$count_429" -gt 0 ]; then
    echo ">>> write tier IS rate-limiting"
else
    echo ">>> FAIL: no 429s -- write tier may not be wired"
fi
echo ""

# =================================================================
#   CLEANUP
# =================================================================

echo "============================================================"
echo "Cleanup: removing burst-test posts (content LIKE '$WRITE_MARKER%')"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    DELETE FROM posts WHERE content LIKE '$WRITE_MARKER%';
" && echo ">>> burst-test posts cleaned up"
echo ""

echo "Done. Buckets will refill within ~1 minute before the next run."

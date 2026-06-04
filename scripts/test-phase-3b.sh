#!/bin/bash
#
# Phase 3b e2e test runner.
# Mints a fresh Clerk JWT via the Backend API, then runs 13 tests in one shot.
# Mint + run happens locally (sub-second), so Clerk's default 60-second JWT
# is plenty.
#
# Usage:
#   chmod +x scripts/test-phase-3b.sh
#   ./scripts/test-phase-3b.sh
#
# To test against a different Clerk user, override the user id at run-time:
#   CLERK_USER_ID=user_xxx ./scripts/test-phase-3b.sh
#
# Requires:
#   - The x-clone-api server running on :8080
#   - .env at the repo root with CLERK_SECRET_KEY filled in
#   - At least one active Clerk session for the user under test
#     (open Clerk dashboard, "Actions > Impersonate user" once if needed)

set -u

# Resolve repo root from this script's location, so the script works no
# matter where it's invoked from.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
BASE_URL="http://localhost:8080"

# Clerk user under test. Override via env var for one-off runs, or change the
# default below for a permanent switch.
MY_CLERK_ID="${CLERK_USER_ID:-user_3EdzrQVevZevoSIOmY5ADRKukMx}"

# --- 0. Load secret and mint a fresh JWT ---
SECRET=$(grep '^CLERK_SECRET_KEY=' "$ENV_FILE" | cut -d= -f2-)
if [ -z "$SECRET" ]; then
    echo "ERROR: could not read CLERK_SECRET_KEY from $ENV_FILE"
    exit 1
fi

# Look up the current active session (the hardcoded one expires fast; this
# always uses whatever session exists right now).
echo "Looking up active session for $MY_CLERK_ID ..."
SESSIONS_RESP=$(curl -s \
    -H "Authorization: Bearer $SECRET" \
    -H "Content-Type: application/json" \
    "https://api.clerk.com/v1/sessions?user_id=$MY_CLERK_ID&status=active")
SESSION_ID=$(echo "$SESSIONS_RESP" | grep -oP '"id":"\Ksess_[^"]+' | head -1)

if [ -z "$SESSION_ID" ]; then
    echo "ERROR: no active session found for user $MY_CLERK_ID"
    echo "Open the Clerk dashboard, click Actions > 'Impersonate user' on the test user once to spawn a session."
    echo "API response:"
    echo "$SESSIONS_RESP"
    exit 1
fi
echo "Using session: $SESSION_ID"

echo "Minting JWT via Clerk Backend API (default template, 60s) ..."
TOKEN_RESP=$(curl -s -X POST \
    -H "Authorization: Bearer $SECRET" \
    -H "Content-Type: application/json" \
    "https://api.clerk.com/v1/sessions/$SESSION_ID/tokens")
JWT=$(echo "$TOKEN_RESP" | grep -oP '"jwt"\s*:\s*"\K[^"]+')

if [ -z "$JWT" ]; then
    echo "ERROR: minting JWT failed. Response:"
    echo "$TOKEN_RESP"
    exit 1
fi
echo "Minted JWT (${#JWT} chars). Running tests immediately ..."
echo ""

AUTH="Authorization: Bearer $JWT"
CT="Content-Type: application/json"

# --- 12. Upload signature (run first, captures the public_id) ---
echo "============================================================"
echo "Test 12: POST /api/upload-signatures (expect 200 + signed params)"
echo "============================================================"
SIG=$(curl -s -X POST -H "$AUTH" "$BASE_URL/api/upload-signatures")
echo "$SIG"
MY_PUBLIC_ID=$(echo "$SIG" | grep -oP '"public_id"\s*:\s*"\K[^"]+')
echo ""
echo ">>> Captured public_id: $MY_PUBLIC_ID"
if echo "$MY_PUBLIC_ID" | grep -q "^x_clone/posts/users/$MY_CLERK_ID/"; then
    echo ">>> ✓ public_id is properly namespaced under this user"
else
    echo ">>> ✗ public_id is NOT properly namespaced — security bug!"
fi
echo ""

# --- 1. Create text-only post ---
echo "============================================================"
echo "Test 1: POST /api/posts (text only) (expect 201)"
echo "============================================================"
CREATE=$(curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":"hello from x-clone batch 2"}' "$BASE_URL/api/posts")
echo "$CREATE" | head -15
POST_ID=$(echo "$CREATE" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
echo ""
echo ">>> Captured POST_ID: $POST_ID"
echo ""

# --- 2. Empty post ---
echo "============================================================"
echo "Test 2: POST /api/posts (empty) (expect 400)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" -H "$CT" -d '{}' "$BASE_URL/api/posts" | head -10
echo ""

# --- 3. Read the post (no auth needed) ---
echo "============================================================"
echo "Test 3: GET /api/posts/\$POST_ID (no auth) (expect 200)"
echo "============================================================"
curl -s -i "$BASE_URL/api/posts/$POST_ID" | head -10
echo ""

# --- 4. Like ---
echo "============================================================"
echo "Test 4: POST /api/posts/\$POST_ID/likes (expect 204)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" "$BASE_URL/api/posts/$POST_ID/likes" | head -5
echo ""

# --- 5. Re-like (idempotent) ---
echo "============================================================"
echo "Test 5: POST /api/posts/\$POST_ID/likes again (expect 204, no error)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" "$BASE_URL/api/posts/$POST_ID/likes" | head -5
echo ""

# --- 6. Unlike ---
echo "============================================================"
echo "Test 6: DELETE /api/posts/\$POST_ID/likes (expect 204)"
echo "============================================================"
curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/posts/$POST_ID/likes" | head -5
echo ""

# --- 7. Like nonexistent post ---
echo "============================================================"
echo "Test 7: POST /api/posts/<nonexistent>/likes (expect 404)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" \
    "$BASE_URL/api/posts/00000000-0000-0000-0000-000000000000/likes" | head -10
echo ""

# --- 8. Delete (no image) ---
echo "============================================================"
echo "Test 8: DELETE /api/posts/\$POST_ID (expect 200)"
echo "============================================================"
curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/posts/$POST_ID" | head -10
echo ""

# --- 9. Get the deleted post ---
echo "============================================================"
echo "Test 9: GET /api/posts/\$POST_ID after delete (expect 404)"
echo "============================================================"
curl -s -i "$BASE_URL/api/posts/$POST_ID" | head -10
echo ""

# --- 10. Foreign Cloudinary URL ---
echo "============================================================"
echo "Test 10: POST /api/posts with FOREIGN URL (expect 400 invalid)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":"trying foreign","image":"https://res.cloudinary.com/dxyz/image/upload/v1/some_random_id.jpg"}' \
    "$BASE_URL/api/posts" | head -10
echo ""

# --- 11. IDOR: another user's namespaced URL ---
echo "============================================================"
echo "Test 11: POST /api/posts with ANOTHER user's URL (expect 400 invalid)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":"trying IDOR","image":"https://res.cloudinary.com/dxyz/image/upload/v1/x_clone/posts/users/user_ATTACKER/somehash.jpg"}' \
    "$BASE_URL/api/posts" | head -10
echo ""

# --- 11b. Own namespaced URL (sanity) ---
echo "============================================================"
echo "Test 11b: POST /api/posts with OWN namespaced URL (expect 201)"
echo "============================================================"
OWN_URL="https://res.cloudinary.com/dxyz/image/upload/v1/$MY_PUBLIC_ID.jpg"
echo "Using URL: $OWN_URL"
CREATE2=$(curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d "{\"content\":\"with my own image\",\"image\":\"$OWN_URL\"}" \
    "$BASE_URL/api/posts")
echo "$CREATE2" | head -15
POST_ID2=$(echo "$CREATE2" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
echo ""
echo ">>> Captured POST_ID2: $POST_ID2"
if [ -n "$POST_ID2" ]; then
    echo ""
    echo "============================================================"
    echo "Test 11c: DELETE the post with own image (exercises destroy path)"
    echo "============================================================"
    curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/posts/$POST_ID2" | head -10
fi

echo ""
echo "============================================================"
echo "Server logs (last 40 lines)"
echo "============================================================"
tail -40 /tmp/x-clone-server.log

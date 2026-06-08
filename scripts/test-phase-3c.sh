#!/bin/bash
#
# Phase 3c e2e test runner.
# Mints a fresh Clerk JWT via the Backend API, then runs ~20 tests covering
# comment writes, profile updates, banner upload signatures, and follow toggle.
# Mint + run happens locally (sub-second), so Clerk's default 60-second JWT
# is plenty.
#
# Usage:
#   chmod +x scripts/test-phase-3c.sh
#   ./scripts/test-phase-3c.sh
#
# To test against a different Clerk user, override the user id at run-time:
#   CLERK_USER_ID=user_xxx ./scripts/test-phase-3c.sh
#
# Requires:
#   - The x-clone-api server running on :8080
#   - psql installed (used to seed/cleanup a synthetic second user for follow tests)
#   - .env at the repo root with CLERK_SECRET_KEY and DATABASE_URL filled in
#   - At least one active Clerk session for the user under test
#     (open Clerk dashboard, "Actions > Impersonate user" once if needed)

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
BASE_URL="http://localhost:8080"

MY_CLERK_ID="${CLERK_USER_ID:-user_3EdzrQVevZevoSIOmY5ADRKukMx}"

# Synthetic target user inserted directly into the DB for follow tests.
# We can't easily spin up a second Clerk user, so we seed a row with
# realistic-looking fields and clean it up at the end.
SYN_CLERK_ID="synthetic_target_3c"
SYN_USERNAME="synthetic_target_3c"
SYN_EMAIL="synthetic_target_3c@example.com"

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

# --- Seed a synthetic target user for follow tests ---
echo "============================================================"
echo "Setup: inserting synthetic target user via psql"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    INSERT INTO users (clerk_id, email, username, first_name, last_name)
    VALUES ('$SYN_CLERK_ID', '$SYN_EMAIL', '$SYN_USERNAME', 'Synthetic', 'Target')
    ON CONFLICT (clerk_id) DO NOTHING;
" && echo ">>> synthetic user ready: $SYN_USERNAME"
echo ""

# Capture my username + bio for self-follow + restore-on-exit
echo "============================================================"
echo "Setup: GET /api/me (capture my username + original bio)"
echo "============================================================"
ME=$(curl -s -H "$AUTH" "$BASE_URL/api/me")
MY_USERNAME=$(echo "$ME" | grep -oP '"username"\s*:\s*"\K[^"]+')
ORIGINAL_BIO=$(echo "$ME" | grep -oP '"bio"\s*:\s*"\K[^"]*')
ORIGINAL_BANNER=$(echo "$ME" | grep -oP '"bannerImage"\s*:\s*"\K[^"]*')
echo ">>> MY_USERNAME=$MY_USERNAME"
echo ">>> ORIGINAL_BIO=$ORIGINAL_BIO"
echo ">>> ORIGINAL_BANNER=$ORIGINAL_BANNER"
echo ""

# =================================================================
#   COMMENT TESTS
# =================================================================

# Create a post we can comment on.
echo "============================================================"
echo "Setup: POST /api/posts (text only) to comment on"
echo "============================================================"
CREATE_POST=$(curl -s -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":"phase 3c parent post"}' "$BASE_URL/api/posts")
POST_ID=$(echo "$CREATE_POST" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
echo ">>> POST_ID=$POST_ID"
echo ""

echo "============================================================"
echo "Test 1: POST /api/posts/\$POST_ID/comments (expect 201)"
echo "============================================================"
CREATE_COMMENT=$(curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":"first comment"}' "$BASE_URL/api/posts/$POST_ID/comments")
echo "$CREATE_COMMENT" | head -12
COMMENT_ID=$(echo "$CREATE_COMMENT" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
echo ">>> COMMENT_ID=$COMMENT_ID"
echo ""

echo "============================================================"
echo "Test 2: POST .../comments with empty content (expect 400)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":""}' "$BASE_URL/api/posts/$POST_ID/comments" | head -8
echo ""

echo "============================================================"
echo "Test 3: POST comment to nonexistent post (expect 404)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d '{"content":"orphan"}' \
    "$BASE_URL/api/posts/00000000-0000-0000-0000-000000000000/comments" | head -8
echo ""

echo "============================================================"
echo "Test 4: GET /api/comments/post/\$POST_ID (expect 200, our comment present)"
echo "============================================================"
curl -s "$BASE_URL/api/comments/post/$POST_ID" | head -c 400
echo ""
echo ""

echo "============================================================"
echo "Test 5: DELETE /api/comments/\$COMMENT_ID (expect 200)"
echo "============================================================"
curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/comments/$COMMENT_ID" | head -8
echo ""

echo "============================================================"
echo "Test 6: DELETE same comment again (expect 404)"
echo "============================================================"
curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/comments/$COMMENT_ID" | head -8
echo ""

# Cleanup: delete the parent post (cascade deletes any remaining comments)
echo "Cleanup: DELETE /api/posts/\$POST_ID"
curl -s -X DELETE -H "$AUTH" "$BASE_URL/api/posts/$POST_ID" > /dev/null
echo ""

# =================================================================
#   PROFILE UPDATE TESTS
# =================================================================

echo "============================================================"
echo "Test 7: PATCH /api/me with empty body (expect 400)"
echo "============================================================"
curl -s -i -X PATCH -H "$AUTH" -H "$CT" -d '{}' "$BASE_URL/api/me" | head -8
echo ""

echo "============================================================"
echo "Test 8: PATCH /api/me {bio: ...} (expect 200, bio updated)"
echo "============================================================"
curl -s -i -X PATCH -H "$AUTH" -H "$CT" \
    -d '{"bio":"phase 3c test bio"}' "$BASE_URL/api/me" | head -12
echo ""

echo "============================================================"
echo "Test 9: PATCH /api/me with foreign banner URL (expect 400)"
echo "============================================================"
curl -s -i -X PATCH -H "$AUTH" -H "$CT" \
    -d '{"bannerImage":"https://res.cloudinary.com/dxyz/image/upload/v1/x_clone/banners/users/user_ATTACKER/somehash.jpg"}' \
    "$BASE_URL/api/me" | head -8
echo ""

echo "============================================================"
echo "Test 10: POST /api/upload-signatures/banners (expect 200)"
echo "============================================================"
SIG=$(curl -s -X POST -H "$AUTH" "$BASE_URL/api/upload-signatures/banners")
echo "$SIG"
BANNER_PUBLIC_ID=$(echo "$SIG" | grep -oP '"public_id"\s*:\s*"\K[^"]+')
echo ">>> BANNER_PUBLIC_ID=$BANNER_PUBLIC_ID"
if echo "$BANNER_PUBLIC_ID" | grep -q "^x_clone/banners/users/$MY_CLERK_ID/"; then
    echo ">>> banner public_id is correctly namespaced under this user"
else
    echo ">>> banner public_id is NOT properly namespaced -- security bug!"
fi
echo ""

echo "============================================================"
echo "Test 11: PATCH /api/me with own banner URL (expect 200)"
echo "============================================================"
OWN_BANNER_URL="https://res.cloudinary.com/dxyz/image/upload/v1/$BANNER_PUBLIC_ID.jpg"
curl -s -i -X PATCH -H "$AUTH" -H "$CT" \
    -d "{\"bannerImage\":\"$OWN_BANNER_URL\"}" "$BASE_URL/api/me" | head -12
echo ""

# Restore original bio + banner so the script is idempotent
echo "Cleanup: restoring original bio + bannerImage"
curl -s -X PATCH -H "$AUTH" -H "$CT" \
    -d "{\"bio\":\"$ORIGINAL_BIO\",\"bannerImage\":\"$ORIGINAL_BANNER\"}" \
    "$BASE_URL/api/me" > /dev/null
echo ""

# =================================================================
#   FOLLOW TOGGLE TESTS
# =================================================================

echo "============================================================"
echo "Test 12: POST /api/users/\$SYN_USERNAME/follow (expect 204)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" "$BASE_URL/api/users/$SYN_USERNAME/follow" | head -5
echo ""

echo "============================================================"
echo "Test 13: POST follow again (expect 204, idempotent)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" "$BASE_URL/api/users/$SYN_USERNAME/follow" | head -5
echo ""

echo "============================================================"
echo "Test 14: POST follow on MY OWN username (expect 400 self-follow)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" "$BASE_URL/api/users/$MY_USERNAME/follow" | head -8
echo ""

echo "============================================================"
echo "Test 15: POST follow on nonexistent user (expect 404)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" "$BASE_URL/api/users/nonexistent_xyz_99/follow" | head -8
echo ""

echo "============================================================"
echo "Test 16: DELETE /api/users/\$SYN_USERNAME/follow (expect 204)"
echo "============================================================"
curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/users/$SYN_USERNAME/follow" | head -5
echo ""

echo "============================================================"
echo "Test 17: DELETE unfollow again (expect 204, idempotent)"
echo "============================================================"
curl -s -i -X DELETE -H "$AUTH" "$BASE_URL/api/users/$SYN_USERNAME/follow" | head -5
echo ""

# =================================================================
#   CLEANUP
# =================================================================

echo "============================================================"
echo "Cleanup: removing synthetic user + any leftover follow rows"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    DELETE FROM user_followers
    WHERE user_id IN (SELECT id FROM users WHERE clerk_id = '$SYN_CLERK_ID')
       OR follower_id IN (SELECT id FROM users WHERE clerk_id = '$SYN_CLERK_ID');
    DELETE FROM users WHERE clerk_id = '$SYN_CLERK_ID';
" && echo ">>> synthetic user cleaned up"
echo ""

echo "============================================================"
echo "Server logs (last 40 lines)"
echo "============================================================"
tail -40 /tmp/x-clone-server.log 2>/dev/null || echo "(no server log at /tmp/x-clone-server.log)"

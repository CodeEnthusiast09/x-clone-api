#!/bin/bash
#
# Phase 5 (REST) e2e test runner.
# Mints a fresh Clerk JWT, seeds a synthetic second user, then tests all
# conversation + message REST endpoints end-to-end.
#
# Usage:
#   chmod +x scripts/test-phase-5-rest.sh
#   ./scripts/test-phase-5-rest.sh
#
# To test against a different Clerk user, override CLERK_USER_ID at run-time:
#   CLERK_USER_ID=user_xxx ./scripts/test-phase-5-rest.sh
#
# Requires:
#   - The x-clone-api server running on :8080
#   - psql installed
#   - .env at the repo root with CLERK_SECRET_KEY and DATABASE_URL filled in
#   - At least one active Clerk session for the test user
#     (open Clerk dashboard → Actions → "Impersonate user" once if needed)

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
BASE_URL="http://localhost:8080"

MY_CLERK_ID="${CLERK_USER_ID:-user_3EdzrQVevZevoSIOmY5ADRKukMx}"

SYN_CLERK_ID="synthetic_target_p5"
SYN_USERNAME="synthetic_target_p5"
SYN_EMAIL="synthetic_target_p5@example.com"

# --- 0. Load env, mint fresh JWT ---
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

echo "Minting JWT ..."
TOKEN_RESP=$(curl -s -X POST \
    -H "Authorization: Bearer $SECRET" \
    -H "Content-Type: application/json" \
    "https://api.clerk.com/v1/sessions/$SESSION_ID/tokens")
JWT=$(echo "$TOKEN_RESP" | grep -oP '"jwt"\s*:\s*"\K[^"]+')

if [ -z "$JWT" ]; then
    echo "ERROR: JWT mint failed. Response: $TOKEN_RESP"
    exit 1
fi
echo "Minted JWT (${#JWT} chars). Running tests ..."
echo ""

AUTH="Authorization: Bearer $JWT"
CT="Content-Type: application/json"

# --- 1. Seed synthetic user ---
echo "============================================================"
echo "Setup: insert synthetic target user"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    INSERT INTO users (clerk_id, email, username, first_name, last_name)
    VALUES ('$SYN_CLERK_ID', '$SYN_EMAIL', '$SYN_USERNAME', 'Synthetic', 'P5')
    ON CONFLICT (clerk_id) DO NOTHING;
" && echo ">>> synthetic user ready: $SYN_USERNAME"

# Capture both DB UUIDs for later psql seeding
MY_DB_ID=$(psql "$DB_URL" -t -A -c "SELECT id FROM users WHERE clerk_id = '$MY_CLERK_ID';")
SYN_DB_ID=$(psql "$DB_URL" -t -A -c "SELECT id FROM users WHERE clerk_id = '$SYN_CLERK_ID';")
echo ">>> MY_DB_ID=$MY_DB_ID"
echo ">>> SYN_DB_ID=$SYN_DB_ID"
echo ""

# =================================================================
#   CONVERSATION TESTS
# =================================================================

echo "============================================================"
echo "Test 1: POST /api/conversations (expect 200, conversation created)"
echo "============================================================"
CONV_RESP=$(curl -s -X POST -H "$AUTH" -H "$CT" \
    -d "{\"recipientId\":\"$SYN_DB_ID\"}" "$BASE_URL/api/conversations")
echo "$CONV_RESP" | head -c 400
CONV_ID=$(echo "$CONV_RESP" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
echo ""
echo ">>> CONV_ID=$CONV_ID"
echo ""

echo "============================================================"
echo "Test 2: POST /api/conversations same pair again (expect 200, idempotent — same CONV_ID)"
echo "============================================================"
CONV_RESP2=$(curl -s -X POST -H "$AUTH" -H "$CT" \
    -d "{\"recipientId\":\"$SYN_DB_ID\"}" "$BASE_URL/api/conversations")
CONV_ID2=$(echo "$CONV_RESP2" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
if [ "$CONV_ID" = "$CONV_ID2" ]; then
    echo ">>> PASS: same conversation returned ($CONV_ID)"
else
    echo ">>> FAIL: got different IDs: $CONV_ID vs $CONV_ID2"
fi
echo ""

echo "============================================================"
echo "Test 3: POST /api/conversations with self (expect 400)"
echo "============================================================"
curl -s -i -X POST -H "$AUTH" -H "$CT" \
    -d "{\"recipientId\":\"$MY_DB_ID\"}" "$BASE_URL/api/conversations" | head -8
echo ""

echo "============================================================"
echo "Test 4: POST /api/conversations without auth (expect 401)"
echo "============================================================"
curl -s -i -X POST -H "$CT" \
    -d "{\"recipientId\":\"$SYN_DB_ID\"}" "$BASE_URL/api/conversations" | head -5
echo ""

echo "============================================================"
echo "Test 5: GET /api/conversations (expect 200, our conversation listed, unreadCount=0)"
echo "============================================================"
LIST_RESP=$(curl -s -H "$AUTH" "$BASE_URL/api/conversations")
echo "$LIST_RESP" | head -c 600
UNREAD=$(echo "$LIST_RESP" | grep -oP '"unreadCount"\s*:\s*\K[0-9]+' | head -1)
echo ""
echo ">>> unreadCount=$UNREAD (want 0)"
echo ""

# =================================================================
#   MESSAGE TESTS (via psql seed — WS send is tested in phase-5-ws.sh)
# =================================================================

echo "============================================================"
echo "Setup: seed one unread message from the synthetic user to me"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    INSERT INTO messages (conversation_id, sender_id, body)
    VALUES ('$CONV_ID', '$SYN_DB_ID', 'hello from synthetic user');
" && echo ">>> message seeded"
echo ""

echo "============================================================"
echo "Test 6: GET /api/conversations (expect unreadCount=1 now)"
echo "============================================================"
LIST_RESP2=$(curl -s -H "$AUTH" "$BASE_URL/api/conversations")
UNREAD2=$(echo "$LIST_RESP2" | grep -oP '"unreadCount"\s*:\s*\K[0-9]+' | head -1)
LAST_BODY=$(echo "$LIST_RESP2" | grep -oP '"body"\s*:\s*"\K[^"]+' | head -1)
echo ">>> unreadCount=$UNREAD2 (want 1)"
echo ">>> lastMessage.body=$LAST_BODY (want 'hello from synthetic user')"
echo ""

echo "============================================================"
echo "Test 7: GET /api/conversations/:conversationId/messages (expect 200, 1 message)"
echo "============================================================"
MSGS_RESP=$(curl -s -H "$AUTH" "$BASE_URL/api/conversations/$CONV_ID/messages")
echo "$MSGS_RESP" | head -c 500
MSG_COUNT=$(echo "$MSGS_RESP" | grep -oP '"total"\s*:\s*\K[0-9]+' | head -1)
MSG_READ_AT=$(echo "$MSGS_RESP" | grep -oP '"readAt"\s*:\s*\K[^,}]+' | head -1)
echo ""
echo ">>> total=$MSG_COUNT (want 1)"
echo ">>> readAt=$MSG_READ_AT (want null)"
echo ""

echo "============================================================"
echo "Test 8: PATCH /api/conversations/:conversationId/read (expect 200, markedRead=1)"
echo "============================================================"
READ_RESP=$(curl -s -i -X PATCH -H "$AUTH" "$BASE_URL/api/conversations/$CONV_ID/read")
echo "$READ_RESP" | head -10
MARKED=$(echo "$READ_RESP" | grep -oP '"markedRead"\s*:\s*\K[0-9]+' | head -1)
echo ">>> markedRead=$MARKED (want 1)"
echo ""

echo "============================================================"
echo "Test 9: PATCH .../read again (expect 200, markedRead=0 — nothing left unread)"
echo "============================================================"
READ_RESP2=$(curl -s -X PATCH -H "$AUTH" "$BASE_URL/api/conversations/$CONV_ID/read")
MARKED2=$(echo "$READ_RESP2" | grep -oP '"markedRead"\s*:\s*\K[0-9]+' | head -1)
echo ">>> markedRead=$MARKED2 (want 0)"
echo ""

echo "============================================================"
echo "Test 10: GET /api/conversations (expect unreadCount=0 after mark-read)"
echo "============================================================"
LIST_RESP3=$(curl -s -H "$AUTH" "$BASE_URL/api/conversations")
UNREAD3=$(echo "$LIST_RESP3" | grep -oP '"unreadCount"\s*:\s*\K[0-9]+' | head -1)
echo ">>> unreadCount=$UNREAD3 (want 0)"
echo ""

echo "============================================================"
echo "Test 11: GET messages for conversation I don't own (expect 404)"
echo "============================================================"
curl -s -i -H "$AUTH" \
    "$BASE_URL/api/conversations/00000000-0000-0000-0000-000000000000/messages" | head -8
echo ""

echo "============================================================"
echo "Test 12: PATCH read for conversation I don't own (expect 404)"
echo "============================================================"
curl -s -i -X PATCH -H "$AUTH" \
    "$BASE_URL/api/conversations/00000000-0000-0000-0000-000000000000/read" | head -8
echo ""

# =================================================================
#   CLEANUP
# =================================================================

echo "============================================================"
echo "Cleanup: removing conversation (cascades messages) and synthetic user"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    DELETE FROM conversations WHERE id = '$CONV_ID';
    DELETE FROM users WHERE clerk_id = '$SYN_CLERK_ID';
" && echo ">>> cleanup done"
echo ""

echo "============================================================"
echo "All Phase 5 REST tests complete."
echo "============================================================"

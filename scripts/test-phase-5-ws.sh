#!/bin/bash
#
# Phase 5 (WebSocket) e2e test runner.
# Mints a fresh Clerk JWT, creates a conversation, then tests the WS endpoint:
#   - auth guard (no token → upgrade rejected)
#   - non-participant guard (wrong conv → 403)
#   - send a message, verify it echoes back with correct shape
#   - verify the message was persisted (REST GET /messages)
#   - verify read receipt fires on connect
#
# Usage:
#   chmod +x scripts/test-phase-5-ws.sh
#   ./scripts/test-phase-5-ws.sh
#
# Requires:
#   - The x-clone-api server running on :8080
#   - psql installed
#   - .env at the repo root with CLERK_SECRET_KEY and DATABASE_URL filled in
#   - At least one active Clerk session for the test user

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
BASE_URL="http://localhost:8080"
WS_BASE="ws://localhost:8080"

MY_CLERK_ID="${CLERK_USER_ID:-user_3EdzrQVevZevoSIOmY5ADRKukMx}"
SYN_CLERK_ID="synthetic_target_ws"
SYN_EMAIL="synthetic_target_ws@example.com"
SYN_USERNAME="synthetic_target_ws"

# --- 0. Mint JWT ---
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
    echo "ERROR: no active session. Impersonate the user via Clerk dashboard first."
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
echo "Minted JWT (${#JWT} chars)."
echo ""

AUTH="Authorization: Bearer $JWT"
CT="Content-Type: application/json"

# --- 1. Seed synthetic user + create conversation ---
echo "============================================================"
echo "Setup: seed synthetic user and create conversation"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    INSERT INTO users (clerk_id, email, username, first_name, last_name)
    VALUES ('$SYN_CLERK_ID', '$SYN_EMAIL', '$SYN_USERNAME', 'Synthetic', 'WS')
    ON CONFLICT (clerk_id) DO NOTHING;
" && echo ">>> synthetic user ready"

SYN_DB_ID=$(psql "$DB_URL" -t -A -c "SELECT id FROM users WHERE clerk_id = '$SYN_CLERK_ID';")
echo ">>> SYN_DB_ID=$SYN_DB_ID"

CONV_RESP=$(curl -s -X POST -H "$AUTH" -H "$CT" \
    -d "{\"recipientId\":\"$SYN_DB_ID\"}" "$BASE_URL/api/conversations")
CONV_ID=$(echo "$CONV_RESP" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
echo ">>> CONV_ID=$CONV_ID"
if [ -z "$CONV_ID" ]; then
    echo "ERROR: could not create conversation. Aborting."
    echo "$CONV_RESP"
    exit 1
fi
echo ""

# --- 2. Seed an unread message (from syn → me) to test read receipts ---
echo "============================================================"
echo "Setup: seed unread message from synthetic user"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    INSERT INTO messages (conversation_id, sender_id, body, created_at, updated_at)
    VALUES ('$CONV_ID', '$SYN_DB_ID', 'unread message before connect', NOW() - interval '5 seconds', NOW() - interval '5 seconds');
" && echo ">>> unread message seeded"
echo ""

# --- 3. WS tests ---
echo "============================================================"
echo "Test 1: WS connect without auth (expect upgrade rejected)"
echo "============================================================"
go run "$REPO_ROOT/scripts/ws_test_client" \
    -url "$WS_BASE/api/conversations/$CONV_ID/ws" \
    -token "invalid_token" \
    -msg "should not reach server" 2>&1 | head -3
echo ""

echo "============================================================"
echo "Test 2: WS connect to conversation I don't own (expect 403)"
echo "============================================================"
go run "$REPO_ROOT/scripts/ws_test_client" \
    -url "$WS_BASE/api/conversations/00000000-0000-0000-0000-000000000000/ws" \
    -token "$JWT" \
    -msg "intruder" 2>&1 | head -3
echo ""

echo "============================================================"
echo "Test 3: WS connect, send message, receive echo (expect OK)"
echo "============================================================"
go run "$REPO_ROOT/scripts/ws_test_client" \
    -url "$WS_BASE/api/conversations/$CONV_ID/ws" \
    -token "$JWT" \
    -msg "hello from ws test"
echo ""

echo "============================================================"
echo "Test 4: Verify message was persisted (REST GET /messages)"
echo "============================================================"
MSGS=$(curl -s -H "$AUTH" "$BASE_URL/api/conversations/$CONV_ID/messages")
MSG_COUNT=$(echo "$MSGS" | grep -oP '"total"\s*:\s*\K[0-9]+' | head -1)
LAST_BODY=$(echo "$MSGS" | grep -oP '"body"\s*:\s*"\K[^"]+' | tail -1)
echo ">>> total=$MSG_COUNT (want 2 — seeded + ws)"
echo ">>> last body=$LAST_BODY (want 'hello from ws test')"
echo ""

echo "============================================================"
echo "Test 5: Verify read receipt — seeded message should now be read"
echo "============================================================"
READ_AT_COUNT=$(psql "$DB_URL" -t -A -c "
    SELECT COUNT(*) FROM messages
    WHERE conversation_id = '$CONV_ID' AND read_at IS NOT NULL;
")
echo ">>> messages with read_at set = $READ_AT_COUNT (want 1 — the seeded unread one)"
echo ""

# --- Cleanup ---
echo "============================================================"
echo "Cleanup"
echo "============================================================"
psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c "
    DELETE FROM conversations WHERE id = '$CONV_ID';
    DELETE FROM users WHERE clerk_id = '$SYN_CLERK_ID';
" && echo ">>> cleanup done"
echo ""

echo "============================================================"
echo "All Phase 5 WS tests complete."
echo "============================================================"

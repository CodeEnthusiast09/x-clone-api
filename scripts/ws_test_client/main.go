// One-shot WebSocket test client used by test-phase-5-ws.sh.
// Usage: go run ./scripts/ws_test_client \
//          -url ws://localhost:8080/api/conversations/<id>/ws \
//          -token <jwt> \
//          -msg  "hello world"
//
// Connects, sends one message, waits for a "message" event back, prints it,
// then exits 0. Exits non-zero on any error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	wsURL := flag.String("url", "", "WebSocket URL")
	token := flag.String("token", "", "Bearer JWT")
	msg := flag.String("msg", "hello from ws test", "Message body to send")
	flag.Parse()

	if *wsURL == "" || *token == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -url and -token are required")
		os.Exit(1)
	}

	header := http.Header{"Authorization": {"Bearer " + *token}}
	conn, resp, err := websocket.DefaultDialer.Dial(*wsURL, header)
	if err != nil {
		if resp != nil {
			fmt.Fprintf(os.Stderr, "ERROR: dial failed (HTTP %d): %v\n", resp.StatusCode, err)
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: dial failed: %v\n", err)
		}
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("CONNECTED")

	// Send the message
	payload, _ := json.Marshal(map[string]string{"body": *msg})
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SENT: %s\n", payload)

	// Wait up to 10 seconds for the echo back
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: read failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("RECEIVED: %s\n", raw)

	// Validate shape
	var evt struct {
		Type string `json:"type"`
		Data struct {
			ID   string `json:"id"`
			Body string `json:"body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &evt); err != nil || evt.Type != "message" || evt.Data.ID == "" {
		fmt.Fprintf(os.Stderr, "ERROR: unexpected event shape: %s\n", raw)
		os.Exit(1)
	}

	fmt.Printf("OK: type=%s id=%s body=%q\n", evt.Type, evt.Data.ID, evt.Data.Body)
}

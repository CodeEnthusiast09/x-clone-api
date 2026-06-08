package chat

// Hub maintains the set of active clients grouped by conversation, and
// broadcasts messages to each room. Run() must be called in its own goroutine.
// All rooms access is confined to the Run() goroutine — no mutex required.
type Hub struct {
	// rooms maps conversationID (string) -> set of connected clients
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMsg
}

// BroadcastMsg is the envelope sent on Hub.broadcast.
type BroadcastMsg struct {
	ConversationID string
	Payload        []byte
	Exclude        *Client // optional — the sender doesn't need an echo
}

// WSEvent is the JSON envelope sent to every connected client.
type WSEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMsg, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			if h.rooms[c.conversationID] == nil {
				h.rooms[c.conversationID] = make(map[*Client]bool)
			}
			h.rooms[c.conversationID][c] = true

		case c := <-h.unregister:
			if room, ok := h.rooms[c.conversationID]; ok {
				if _, ok := room[c]; ok {
					delete(room, c)
					close(c.send)
					if len(room) == 0 {
						delete(h.rooms, c.conversationID)
					}
				}
			}

		case msg := <-h.broadcast:
			for c := range h.rooms[msg.ConversationID] {
				if c == msg.Exclude {
					continue
				}
				select {
				case c.send <- msg.Payload:
				default:
					// Slow client — drop connection
					close(c.send)
					delete(h.rooms[msg.ConversationID], c)
				}
			}
		}
	}
}

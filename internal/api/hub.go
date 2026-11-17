package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/puvvadi/dash/internal/store"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]clientFilter
}

type clientFilter struct {
	Category string
	Group    string
	Tag      string
}

func NewHub() *Hub {
	return &Hub{clients: map[*websocket.Conn]clientFilter{}}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (h *Hub) Broadcast(snap store.Snapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	msg := map[string]any{"type": "snapshot", "data": snap}
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for c := range h.clients {
		_ = c.WriteMessage(websocket.TextMessage, b)
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, getSnap func() store.Snapshot) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = clientFilter{}
	h.mu.Unlock()

	snap := getSnap()
	init, _ := json.Marshal(map[string]any{"type": "snapshot", "data": snap})
	_ = conn.WriteMessage(websocket.TextMessage, init)

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			_ = conn.Close()
		}()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg map[string]any
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			switch msg["type"] {
			case "ping":
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
			case "subscribe":
				f := clientFilter{}
				if v, ok := msg["category"].(string); ok {
					f.Category = v
				}
				if v, ok := msg["group"].(string); ok {
					f.Group = v
				}
				if v, ok := msg["tag"].(string); ok {
					f.Tag = v
				}
				h.mu.Lock()
				h.clients[conn] = f
				h.mu.Unlock()
			}
		}
	}()
}

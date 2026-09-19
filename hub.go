package hub

import (
	"context"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// Hub keeps one Redis subscriber per poll (not one per browser tab) and
// fans each published update out to every connected websocket client for
// that poll. This is the piece that makes Redis do "real work": Mongo
// stores the poll definitions, Redis carries the live vote stream.
type Hub struct {
	rdb   *redis.Client
	mu    sync.Mutex
	rooms map[string]*room
}

type room struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
	cancel  context.CancelFunc
}

func New(rdb *redis.Client) *Hub {
	return &Hub{
		rdb:   rdb,
		rooms: make(map[string]*room),
	}
}

func (h *Hub) Register(pollID string, conn *websocket.Conn) {
	h.mu.Lock()
	r, ok := h.rooms[pollID]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		r = &room{clients: make(map[*websocket.Conn]bool), cancel: cancel}
		h.rooms[pollID] = r
		go h.subscribeLoop(ctx, pollID, r)
	}
	h.mu.Unlock()

	r.mu.Lock()
	r.clients[conn] = true
	r.mu.Unlock()
}

func (h *Hub) Unregister(pollID string, conn *websocket.Conn) {
	h.mu.Lock()
	r, ok := h.rooms[pollID]
	h.mu.Unlock()
	if !ok {
		return
	}

	r.mu.Lock()
	delete(r.clients, conn)
	empty := len(r.clients) == 0
	r.mu.Unlock()

	if empty {
		h.mu.Lock()
		// double-check nobody joined in between
		if r2, ok := h.rooms[pollID]; ok {
			r2.mu.Lock()
			stillEmpty := len(r2.clients) == 0
			r2.mu.Unlock()
			if stillEmpty {
				r2.cancel()
				delete(h.rooms, pollID)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) subscribeLoop(ctx context.Context, pollID string, r *room) {
	channel := "poll:" + pollID + ":updates"
	sub := h.rdb.Subscribe(ctx, channel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			r.broadcast([]byte(msg.Payload))
		}
	}
}

func (rm *room) broadcast(payload []byte) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for conn := range rm.clients {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("ws write error, dropping client: %v", err)
			conn.Close()
			delete(rm.clients, conn)
		}
	}
}

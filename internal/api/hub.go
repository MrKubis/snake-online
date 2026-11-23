package api

import (
	"context"
	"strings"
	"sync"
)

type Hub struct {
	mu          sync.RWMutex
	connections map[string]chan string // playerId -> channel
}

func (h *Hub) Register(id string) chan string {
	ch := make(chan string, 1)
	h.mu.Lock()
	h.connections[id] = ch
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	delete(h.connections, id)
	h.mu.Unlock()
}

func (s *Server) RunHub(ctx context.Context) {
	sub := s.rdb.Subscribe(ctx, "player:*")
	defer sub.Close()
	for m := range sub.Channel() {
		id := strings.TrimPrefix(m.Channel, "player:")
		s.hub.mu.RLock()
		ch, ok := s.hub.connections[id]
		s.hub.mu.RUnlock()
		if ok {
			select {
			case ch <- m.Payload:
			default:
			}
		}
	}
}

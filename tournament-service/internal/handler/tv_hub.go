package handler

import "sync"

// Hub — SSE push uchun token → client kanallar xaritasi
type Hub struct {
	mu      sync.Mutex
	clients map[string]map[chan struct{}]bool
}

func newHub() *Hub {
	return &Hub{clients: make(map[string]map[chan struct{}]bool)}
}

func (h *Hub) subscribe(token string) chan struct{} {
	ch := make(chan struct{}, 4)
	h.mu.Lock()
	if h.clients[token] == nil {
		h.clients[token] = make(map[chan struct{}]bool)
	}
	h.clients[token][ch] = true
	h.mu.Unlock()
	return ch
}

func (h *Hub) unsubscribe(token string, ch chan struct{}) {
	h.mu.Lock()
	delete(h.clients[token], ch)
	if len(h.clients[token]) == 0 {
		delete(h.clients, token)
	}
	h.mu.Unlock()
}

// Broadcast — token ga ulangan barcha clientlarga refresh signali yuboradi
func (h *Hub) Broadcast(token string) {
	h.mu.Lock()
	clients := make([]chan struct{}, 0, len(h.clients[token]))
	for ch := range h.clients[token] {
		clients = append(clients, ch)
	}
	h.mu.Unlock()

	for _, ch := range clients {
		select {
		case ch <- struct{}{}:
		default: // sekin client — o'tkazib yuboramiz, bloklanmaydi
		}
	}
}

// Count — token ga ulangan clientlar soni
func (h *Hub) Count(token string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[token])
}

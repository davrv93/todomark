package presence

import "sync"

// Hub cuenta clientes conectados vía SSE y les avisa el total cada vez que cambia.
type Hub struct {
	mu      sync.Mutex
	clients map[chan int]struct{}
	count   int
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan int]struct{})}
}

func (h *Hub) Add() chan int {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan int, 1)
	h.clients[ch] = struct{}{}
	h.count++
	h.broadcastLocked()
	return ch
}

func (h *Hub) Remove(ch chan int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; !ok {
		return
	}
	delete(h.clients, ch)
	close(ch)
	h.count--
	h.broadcastLocked()
}

func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

// broadcastLocked asume que el caller ya tiene h.mu. No bloquea si un cliente
// está saturado (buffer 1): se salta ese envío, el próximo cambio lo alcanza.
func (h *Hub) broadcastLocked() {
	for ch := range h.clients {
		select {
		case ch <- h.count:
		default:
		}
	}
}

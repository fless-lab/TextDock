// Package events broadcasts invalidations, never SMS content or credentials.
// Reconnection always requires a fresh scoped read, so no replay buffer is needed.
package events

import "sync"

type Filter struct{ To, RunID, DeviceID string }
type subscriber struct {
	filter Filter
	ch     chan string
}
type Hub struct {
	mu      sync.Mutex
	clients map[*subscriber]struct{}
}

func (h *Hub) Subscribe(filter Filter) (<-chan string, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients == nil {
		h.clients = make(map[*subscriber]struct{})
	}
	s := &subscriber{filter: filter, ch: make(chan string, 1)}
	h.clients[s] = struct{}{}
	return s.ch, func() { h.mu.Lock(); defer h.mu.Unlock(); delete(h.clients, s) }
}

func (h *Hub) Changed(to, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.clients {
		if s.filter.To != "" && s.filter.To != to {
			continue
		}
		if s.filter.RunID != "" && s.filter.RunID != runID {
			continue
		}
		select {
		case s.ch <- "sync":
		default:
		}
	}
}

func (h *Hub) Revoke(deviceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.clients {
		if s.filter.DeviceID == deviceID {
			// Revocation takes priority over a coalesced invalidation.
			select {
			case <-s.ch:
			default:
			}
			s.ch <- "revoked"
		}
	}
}

package realtime

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// sendBuffer is how many events may queue for one connection before it is
// considered too slow and dropped. Small on purpose: a healthy tab drains
// instantly, and a wedged one should not accumulate memory.
const sendBuffer = 32

// Hub keeps the live connections grouped by user. It is safe for concurrent use.
//
// This is intentionally an in-process hub: the deployment runs a single app
// container, so there is no need for cross-process fan-out. Scaling out would
// require a shared bus (Redis pub/sub or Postgres LISTEN/NOTIFY).
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
	logger  *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[int64]map[*Client]struct{}),
		logger:  logger,
	}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.clients[c.userID]
	if !ok {
		conns = make(map[*Client]struct{})
		h.clients[c.userID] = conns
	}
	conns[c] = struct{}{}
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.clients[c.userID]
	if !ok {
		return
	}
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.clients, c.userID)
	}
}

// BroadcastUser delivers ev to every connection of the given users, regardless of
// which project each connection was opened under. Use it for notices that concern
// the person rather than a project they are currently viewing, such as "you were
// added to a project".
func (h *Hub) BroadcastUser(userIDs []int64, ev Event) {
	if len(userIDs) == 0 {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		h.logger.Error("marshal realtime event", "type", ev.Type, "error", err)
		return
	}

	seen := make(map[int64]struct{}, len(userIDs))

	h.mu.RLock()
	targets := make([]*Client, 0, len(userIDs))
	for _, id := range userIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		for c := range h.clients[id] {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.deliver(c, payload, ev.Type)
	}
}

// deliver queues one payload for a client, dropping it if the tab is wedged.
func (h *Hub) deliver(c *Client, payload []byte, eventType string) {
	select {
	case c.send <- payload:
	default:
		// Buffer full: the tab is not draining. Drop this event for it
		// rather than blocking the request that is broadcasting.
		h.logger.Warn("realtime client backlogged, dropping event",
			"user_id", c.userID, "type", eventType)
	}
}

// Broadcast delivers ev to every open connection of the given users. Delivery is
// best-effort and never blocks the caller: a connection whose buffer is full is
// dropped so a stalled tab cannot stall a bill save.
//
// A connection only receives events for the project it was opened under. A user
// with two tabs on two projects therefore gets each event on the matching tab
// only, and another tenant's payload never reaches a socket at all.
func (h *Hub) Broadcast(userIDs []int64, ev Event) {
	if len(userIDs) == 0 {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		h.logger.Error("marshal realtime event", "type", ev.Type, "error", err)
		return
	}

	seen := make(map[int64]struct{}, len(userIDs))

	h.mu.RLock()
	targets := make([]*Client, 0, len(userIDs))
	for _, id := range userIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		for c := range h.clients[id] {
			// An event with no project is delivered everywhere: those are the
			// socket-level signals rather than tenant data.
			if ev.ProjectID != 0 && c.projectID != ev.ProjectID {
				continue
			}
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.deliver(c, payload, ev.Type)
	}
}

// Count reports the number of open connections, used by tests and health checks.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, conns := range h.clients {
		total += len(conns)
	}
	return total
}

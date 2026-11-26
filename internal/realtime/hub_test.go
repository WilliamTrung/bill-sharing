package realtime

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"
)

func testHub() *Hub {
	return NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func testClient(h *Hub, userID int64) *Client {
	// Project 1 by default; NewEvent's events carry no project id, so they reach
	// every connection regardless.
	c := &Client{userID: userID, projectID: 1, send: make(chan []byte, sendBuffer)}
	h.add(c)
	return c
}

// projectClient is a connection opened under a specific project.
func projectClient(h *Hub, userID, projectID int64) *Client {
	c := &Client{userID: userID, projectID: projectID, send: make(chan []byte, sendBuffer)}
	h.add(c)
	return c
}

func drain(c *Client) (Event, bool) {
	select {
	case raw := <-c.send:
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			return Event{}, false
		}
		return ev, true
	default:
		return Event{}, false
	}
}

func TestBroadcastReachesOnlyRecipients(t *testing.T) {
	h := testHub()
	in := testClient(h, 1)
	out := testClient(h, 2)

	h.Broadcast([]int64{1}, NewEvent(TypeBillCreated, &BillEvent{BillID: 9, Title: "Lunch"}, nil))

	if ev, ok := drain(in); !ok || ev.Type != TypeBillCreated {
		t.Fatalf("recipient got %+v (ok=%v)", ev, ok)
	}
	if _, ok := drain(out); ok {
		t.Fatal("non-recipient received an event")
	}
}

func TestBroadcastReachesEveryConnectionOfAUser(t *testing.T) {
	h := testHub()
	tabA := testClient(h, 7)
	tabB := testClient(h, 7)

	h.Broadcast([]int64{7}, NewEvent(TypeGroupCreated, nil, &GroupEvent{GroupID: 3, Name: "Couple"}))

	if _, ok := drain(tabA); !ok {
		t.Fatal("first tab missed the event")
	}
	if _, ok := drain(tabB); !ok {
		t.Fatal("second tab missed the event")
	}
	if h.Count() != 2 {
		t.Fatalf("Count = %d, want 2", h.Count())
	}
}

func TestBroadcastDeduplicatesRecipientIDs(t *testing.T) {
	h := testHub()
	c := testClient(h, 4)

	h.Broadcast([]int64{4, 4, 4}, NewEvent(TypeBillUpdated, &BillEvent{BillID: 1}, nil))

	if _, ok := drain(c); !ok {
		t.Fatal("expected one event")
	}
	if _, ok := drain(c); ok {
		t.Fatal("duplicate recipient id produced a second event")
	}
}

func TestBroadcastToFullClientDoesNotBlock(t *testing.T) {
	h := testHub()
	c := testClient(h, 5)
	// Fill the buffer without a reader.
	for i := 0; i < sendBuffer; i++ {
		c.send <- []byte(`{}`)
	}

	done := make(chan struct{})
	go func() {
		h.Broadcast([]int64{5}, NewEvent(TypeBillCreated, &BillEvent{BillID: 1}, nil))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked on a saturated client")
	}
}

func TestRemoveDropsUser(t *testing.T) {
	h := testHub()
	c := testClient(h, 8)
	h.remove(c)

	if h.Count() != 0 {
		t.Fatalf("Count = %d after remove, want 0", h.Count())
	}

	h.Broadcast([]int64{8}, NewEvent(TypeBillCreated, &BillEvent{BillID: 1}, nil))
	if _, ok := drain(c); ok {
		t.Fatal("removed client still received an event")
	}
}

// A user with tabs on two projects must only receive each event on the tab that
// is showing that project. This is the tenant boundary at the socket layer.
func TestBroadcastIsScopedToConnectionProject(t *testing.T) {
	h := testHub()
	alpha := projectClient(h, 1, 10)
	beta := projectClient(h, 1, 20)

	ev := NewEvent(TypeBillCreated, &BillEvent{BillID: 5, Title: "Alpha"}, nil)
	ev.ProjectID = 10
	h.Broadcast([]int64{1}, ev)

	if got, ok := drain(alpha); !ok || got.ProjectID != 10 {
		t.Fatalf("Alpha tab got %+v (ok=%v)", got, ok)
	}
	if got, ok := drain(beta); ok {
		t.Fatalf("Beta tab received a project-10 event: %+v", got)
	}
}

// An event with no project id is a socket-level signal and reaches every tab.
func TestBroadcastWithoutProjectReachesAllTabs(t *testing.T) {
	h := testHub()
	a := projectClient(h, 3, 10)
	b := projectClient(h, 3, 20)

	h.Broadcast([]int64{3}, NewEvent(TypeBillUpdated, &BillEvent{BillID: 1}, nil))

	if _, ok := drain(a); !ok {
		t.Fatal("first tab missed the unscoped event")
	}
	if _, ok := drain(b); !ok {
		t.Fatal("second tab missed the unscoped event")
	}
}

func TestEventJSONRoundTrip(t *testing.T) {
	ev := NewEvent(TypeBillSettled, &BillEvent{
		BillID: 11, Title: "Dinner", AmountCents: 4200, ActorID: 2, ActorName: "Alex", Status: "OPEN",
	}, nil)

	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var back Event
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Type != ev.Type || back.Bill == nil || back.Bill.BillID != 11 || back.Group != nil {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	if back.At.IsZero() {
		t.Fatal("timestamp was not preserved")
	}
}

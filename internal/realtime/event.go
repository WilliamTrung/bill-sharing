package realtime

import "time"

// Event type names. Clients switch on these; unknown values are ignored.
const (
	TypeBillCreated         = "bill.created"
	TypeBillUpdated         = "bill.updated"
	TypeBillSettled         = "bill.settled"
	TypeBillSettleRequested = "bill.settle_requested"
	TypeBillSettleRejected  = "bill.settle_rejected"
	TypeGroupCreated        = "group.created"
	TypeGroupUpdated        = "group.updated"
)

// Event is one message pushed to connected clients. Payloads stay small on
// purpose: clients refetch the authoritative data and only use these to know
// what changed.
type Event struct {
	Type  string      `json:"type"`
	At    time.Time   `json:"at"`
	Bill  *BillEvent  `json:"bill,omitempty"`
	Group *GroupEvent `json:"group,omitempty"`
}

type BillEvent struct {
	BillID      int64  `json:"bill_id"`
	Title       string `json:"title"`
	AmountCents int64  `json:"total_amount_cents"`
	ActorID     int64  `json:"actor_id,omitempty"`
	ActorName   string `json:"actor_name,omitempty"`
	Status      string `json:"status,omitempty"`
}

type GroupEvent struct {
	GroupID   int64  `json:"group_id"`
	Name      string `json:"name"`
	ActorName string `json:"actor_name,omitempty"`
}

// NewEvent stamps the current time and copies the payload pointer.
func NewEvent(kind string, bill *BillEvent, group *GroupEvent) Event {
	return Event{Type: kind, At: time.Now().UTC(), Bill: bill, Group: group}
}

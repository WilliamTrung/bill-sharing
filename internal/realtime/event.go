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
	// Project membership events. A client that is viewing a different project
	// still records the bell entry but does not refresh its page.
	TypeProjectMemberAdded   = "project.member_added"
	TypeProjectMemberRemoved = "project.member_removed"
)

// Event is one message pushed to connected clients. Payloads stay small on
// purpose: clients refetch the authoritative data and only use these to know
// what changed.
//
// ProjectID is what lets a tab ignore events for a project it is not showing. A
// user may have several tabs open on different projects, so the hub stays keyed
// by user and the client filters.
type Event struct {
	Type      string        `json:"type"`
	At        time.Time     `json:"at"`
	ProjectID int64         `json:"project_id,omitempty"`
	Bill      *BillEvent    `json:"bill,omitempty"`
	Group     *GroupEvent   `json:"group,omitempty"`
	Project   *ProjectEvent `json:"project,omitempty"`
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

type ProjectEvent struct {
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
}

// NewEvent stamps the current time and copies the payload pointer.
func NewEvent(kind string, bill *BillEvent, group *GroupEvent) Event {
	return Event{Type: kind, At: time.Now().UTC(), Bill: bill, Group: group}
}

// NewProjectEvent builds a project membership event.
func NewProjectEvent(kind string, projectID int64, name string) Event {
	return Event{
		Type:      kind,
		At:        time.Now().UTC(),
		ProjectID: projectID,
		Project:   &ProjectEvent{ProjectID: projectID, Name: name},
	}
}

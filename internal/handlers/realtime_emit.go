package handlers

import (
	"context"

	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/WilliamTrung/bill-sharing/internal/realtime"
)

// billParticipantIDs returns every member who owes a share on a bill — the set
// of people who should hear about changes to it. Because overlapping selections
// are rejected, the shares are exactly the distinct participants, including
// members that came in through a group.
//
// The join through bills keeps the fan-out self-defending: even if a caller
// passed a bill id from another project, only members of that bill's own project
// can be resolved as recipients.
func (s *Server) billParticipantIDs(ctx context.Context, projectID, billID int64) ([]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT s.member_id
		FROM bill_member_shares s
		JOIN bills b ON b.id = s.bill_id
		WHERE s.bill_id = $1 AND b.project_id = $2`, billID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0, 8)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// displayName resolves the label shown for a member inside a notification.
func (s *Server) displayName(ctx context.Context, id int64) string {
	var name string
	if err := s.db.QueryRow(ctx, `SELECT `+userLabelSQL+` FROM users WHERE id = $1`, id).Scan(&name); err != nil {
		return ""
	}
	return name
}

// billEventInfo loads the small bill summary a notification needs. A failure is
// non-fatal: realtime delivery must never break the write that triggered it.
// The project predicate keeps another tenant's title and amount off the wire.
func (s *Server) billEventInfo(ctx context.Context, projectID, billID int64) realtime.BillEvent {
	info := realtime.BillEvent{BillID: billID}
	_ = s.db.QueryRow(ctx, `
		SELECT title, total_amount_cents, status FROM bills WHERE id = $1 AND project_id = $2`,
		billID, projectID,
	).Scan(&info.Title, &info.AmountCents, &info.Status)
	return info
}

// emitBill pushes one bill event to the given recipients.
func (s *Server) emitBill(projectID int64, recipients []int64, kind string, info realtime.BillEvent) {
	ev := realtime.NewEvent(kind, &info, nil)
	ev.ProjectID = projectID
	s.hub.Broadcast(recipients, ev)
}

// emitGroup pushes one group event to the group's members.
func (s *Server) emitGroup(projectID int64, memberIDs []int64, kind string, info realtime.GroupEvent) {
	ev := realtime.NewEvent(kind, nil, &info)
	ev.ProjectID = projectID
	s.hub.Broadcast(memberIDs, ev)
}

// emitProject pushes a membership event.
//
// This deliberately bypasses the per-project socket filter: the point is to tell
// someone they just joined a project, and their open tab is almost certainly on a
// different one. It is a notice about the person, not about a project they are
// currently looking at.
func (s *Server) emitProject(recipients []int64, kind string, projectID int64, name string) {
	s.hub.BroadcastUser(recipients, realtime.NewProjectEvent(kind, projectID, name))
}

// excludingID returns ids without the given one, so an actor does not notify
// themselves about their own action.
func excludingID(ids []int64, skip int64) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != skip {
			out = append(out, id)
		}
	}
	return out
}

// notifyShareChange tells the counterparty about a settlement action and nudges
// the rest of the bill to re-render. `targetID` is the person who needs to act:
// the payer for "request", otherwise the debtor whose share changed.
func (s *Server) notifyShareChange(ctx context.Context, projectID, billID, targetID int64, action string, actorID int64) {
	info := s.billEventInfo(ctx, projectID, billID)
	info.ActorID = actorID
	info.ActorName = s.displayName(ctx, actorID)

	var kind string
	switch action {
	case "request":
		kind = realtime.TypeBillSettleRequested
	case "settle":
		kind = realtime.TypeBillSettled
	default: // reject / reopen
		kind = realtime.TypeBillSettleRejected
	}

	if targetID != 0 {
		recipients := []int64{targetID}
		if action == "reopen" {
			// A reopen only needs the page to refresh, not a bell entry.
			s.emitBill(projectID, recipients, realtime.TypeBillUpdated, info)
		} else {
			s.emitBill(projectID, recipients, kind, info)
		}
	}

	// Everyone else on the bill just re-renders; they do not need a direct alert.
	others, err := s.billParticipantIDs(ctx, projectID, billID)
	if err != nil {
		s.logger.Error("bill recipients", "bill_id", billID, "error", err)
		return
	}
	others = excludingID(others, targetID)
	others = excludingID(others, actorID)
	s.emitBill(projectID, others, realtime.TypeBillUpdated, info)
}

// notifyGroupMembers tells every member of a group that it changed. `group` is
// the freshly loaded group, so its member list is current.
func (s *Server) notifyGroupMembers(ctx context.Context, projectID int64, group models.Group, kind string) {
	if len(group.Members) == 0 {
		return
	}
	ids := make([]int64, 0, len(group.Members))
	for _, member := range group.Members {
		ids = append(ids, member.MemberID)
	}
	s.emitGroup(projectID, ids, kind, realtime.GroupEvent{GroupID: group.ID, Name: group.Name})
}

// removeBillUploads deletes a bill's attachment directory. Shared by bill and
// project deletion so the on-disk layout is handled in one place.
func (s *Server) removeBillUploads(billID int64) {
	if err := s.removeUploadDir(billID); err != nil {
		s.logger.Warn("remove bill uploads", "bill_id", billID, "error", err)
	}
}

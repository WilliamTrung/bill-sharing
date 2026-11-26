package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WilliamTrung/bill-sharing/internal/models"
	"github.com/WilliamTrung/bill-sharing/internal/realtime"
	"github.com/WilliamTrung/bill-sharing/internal/service"
	"github.com/jackc/pgx/v5"
)

const maxAttachments = 10

var allowedUploads = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

type participantInput struct {
	Type   string  `json:"type"`
	ID     int64   `json:"id"`
	Weight float64 `json:"weight"`
	Note   string  `json:"note"`
}

type billPayload struct {
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	TotalAmountCents int64              `json:"total_amount_cents"`
	PaidByMemberID   int64              `json:"paid_by_member_id"`
	SplitMode        string             `json:"split_mode"`
	BillDate         string             `json:"bill_date"`
	Participants     []participantInput `json:"participants"`
}

type resolvedGroup struct {
	ID                int64
	Name              string
	InternalSplitType string
	Members           []resolvedGroupMember
}

type resolvedGroupMember struct {
	MemberID int64
	Name     string
	Ratio    float64
}

type plannedParticipant struct {
	Type      string
	MemberID  *int64
	GroupID   *int64
	Note      string
	Weight    float64
	Allocated int64
}

type plannedShare struct {
	MemberID      int64
	SourceGroupID *int64
	Owed          int64
	// InPayerGroup is true when this share comes from a group that contains the
	// payer, so the whole group is considered already paid.
	InPayerGroup bool
}

type billPlan struct {
	Title        string
	Description  string
	TotalCents   int64
	PayerID      int64
	SplitMode    string
	BillDate     time.Time
	Participants []plannedParticipant
	Shares       []plannedShare
}

type preparedAttachment struct {
	header   *multipart.FileHeader
	mimeType string
	ext      string
	original string
	size     int64
}

func (s *Server) handleCreateBill(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+2<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid upload: "+err.Error())
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	rawPayload := r.FormValue("payload")
	if rawPayload == "" {
		writeError(w, http.StatusBadRequest, "Missing bill payload")
		return
	}
	var payload billPayload
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid bill payload: "+err.Error())
		return
	}

	// Default the payer to the signed-in user unless one was chosen explicitly.
	if payload.PaidByMemberID <= 0 {
		if userID, ok := s.auth.RequestUser(r); ok {
			payload.PaidByMemberID = userID
		}
	}

	attachments, err := s.prepareAttachments(r.MultipartForm.File["files"])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	plan, err := s.buildBillPlan(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	billID, err := s.storeBill(r.Context(), plan, attachments)
	if err != nil {
		s.logger.Error("create bill", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not create bill")
		return
	}

	// Tell every participant the bill exists so their list and detail views update.
	info := s.billEventInfo(r.Context(), billID)
	recipients, err := s.billParticipantIDs(r.Context(), billID)
	if err != nil {
		s.logger.Error("bill recipients", "bill_id", billID, "error", err)
	} else {
		s.emitBill(recipients, realtime.TypeBillCreated, info)
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": billID})
}

func (s *Server) buildBillPlan(ctx context.Context, payload billPayload) (*billPlan, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return nil, badRequestf("Bill title is required")
	}
	if len([]rune(title)) > 200 {
		return nil, badRequestf("Bill title must be 200 characters or fewer")
	}
	description := strings.TrimSpace(payload.Description)
	if len([]rune(description)) > 2000 {
		return nil, badRequestf("Description must be 2000 characters or fewer")
	}
	if payload.TotalAmountCents <= 0 {
		return nil, badRequestf("Total amount must be greater than zero")
	}
	if payload.TotalAmountCents > 100_000_000_000 {
		return nil, badRequestf("Total amount exceeds the supported maximum")
	}

	billDate, err := time.Parse("2006-01-02", strings.TrimSpace(payload.BillDate))
	if err != nil {
		return nil, badRequestf("Bill date must be a valid date (YYYY-MM-DD)")
	}

	splitMode := strings.ToUpper(strings.TrimSpace(payload.SplitMode))
	if splitMode == "" {
		splitMode = "EVEN"
	}
	if splitMode != "EVEN" && splitMode != "RATIO" {
		return nil, badRequestf("Split mode must be EVEN or RATIO")
	}

	if len(payload.Participants) == 0 {
		return nil, badRequestf("Select at least one participant")
	}
	if len(payload.Participants) > 100 {
		return nil, badRequestf("Too many participants")
	}

	var payerActive bool
	err = s.db.QueryRow(ctx, `SELECT is_active FROM users WHERE id = $1`, payload.PaidByMemberID).Scan(&payerActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, badRequestf("Payer does not exist")
	}
	if err != nil {
		return nil, fmt.Errorf("load payer: %w", err)
	}
	if !payerActive {
		return nil, badRequestf("Payer is inactive")
	}

	seenParticipants := make(map[string]bool, len(payload.Participants))
	memberIDs := make([]int64, 0, len(payload.Participants))
	groupIDs := make([]int64, 0, len(payload.Participants))
	for _, p := range payload.Participants {
		participantType := strings.ToUpper(strings.TrimSpace(p.Type))
		if participantType != "MEMBER" && participantType != "GROUP" {
			return nil, badRequestf("Participant type must be MEMBER or GROUP")
		}
		if p.ID <= 0 {
			return nil, badRequestf("Invalid participant")
		}
		key := fmt.Sprintf("%s:%d", participantType, p.ID)
		if seenParticipants[key] {
			return nil, badRequestf("A participant is selected more than once")
		}
		seenParticipants[key] = true
		if participantType == "MEMBER" {
			memberIDs = append(memberIDs, p.ID)
		} else {
			groupIDs = append(groupIDs, p.ID)
		}
	}

	members, err := s.loadMemberRefs(ctx, memberIDs)
	if err != nil {
		return nil, err
	}
	groups, err := s.loadResolvedGroups(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	names := make(map[int64]string, len(members))
	for id, member := range members {
		names[id] = member.Name
	}
	groupSelections := make([]service.GroupSelection, 0, len(groups))
	for _, id := range groupIDs {
		group := groups[id]
		for _, member := range group.Members {
			names[member.MemberID] = member.Name
		}
		memberIDsInGroup := make([]int64, 0, len(group.Members))
		for _, member := range group.Members {
			memberIDsInGroup = append(memberIDsInGroup, member.MemberID)
		}
		groupSelections = append(groupSelections, service.GroupSelection{
			Name:      group.Name,
			MemberIDs: memberIDsInGroup,
		})
	}

	if err := service.ValidateOverlap(memberIDs, groupSelections, names); err != nil {
		return nil, err
	}

	weights := make([]float64, len(payload.Participants))
	for i, p := range payload.Participants {
		participantType := strings.ToUpper(strings.TrimSpace(p.Type))
		if splitMode == "EVEN" {
			weights[i] = 1
			continue
		}
		if p.Weight <= 0 || p.Weight > 1_000_000 {
			if participantType == "GROUP" {
				return nil, badRequestf("Weight for group %q must be greater than zero", groups[p.ID].Name)
			}
			return nil, badRequestf("Weight for %q must be greater than zero", members[p.ID].Name)
		}
		weights[i] = p.Weight
	}

	topShares, err := service.Allocate(payload.TotalAmountCents, weights)
	if err != nil {
		return nil, err
	}

	plan := &billPlan{
		Title:        title,
		Description:  description,
		TotalCents:   payload.TotalAmountCents,
		PayerID:      payload.PaidByMemberID,
		SplitMode:    splitMode,
		BillDate:     billDate,
		Participants: make([]plannedParticipant, 0, len(payload.Participants)),
		Shares:       make([]plannedShare, 0),
	}

	for i, p := range payload.Participants {
		participantType := strings.ToUpper(strings.TrimSpace(p.Type))
		planned := plannedParticipant{
			Type:      participantType,
			Note:      strings.TrimSpace(p.Note),
			Weight:    weights[i],
			Allocated: topShares[i],
		}

		if participantType == "MEMBER" {
			memberID := p.ID
			planned.MemberID = &memberID
			plan.Participants = append(plan.Participants, planned)
			plan.Shares = append(plan.Shares, plannedShare{
				MemberID: memberID,
				Owed:     topShares[i],
			})
			continue
		}

		group := groups[p.ID]
		groupID := p.ID
		planned.GroupID = &groupID
		plan.Participants = append(plan.Participants, planned)

		groupWeights := make([]float64, len(group.Members))
		for j, member := range group.Members {
			if group.InternalSplitType == "RATIO" {
				groupWeights[j] = member.Ratio
			} else {
				groupWeights[j] = 1
			}
		}
		groupShares, err := service.Allocate(topShares[i], groupWeights)
		if err != nil {
			return nil, err
		}
		// A group that includes the payer is settled as a whole: the payer already
		// covered that money.
		groupHasPayer := false
		for _, member := range group.Members {
			if member.MemberID == plan.PayerID {
				groupHasPayer = true
				break
			}
		}
		for j, member := range group.Members {
			sourceGroupID := group.ID
			plan.Shares = append(plan.Shares, plannedShare{
				MemberID:      member.MemberID,
				SourceGroupID: &sourceGroupID,
				Owed:          groupShares[j],
				InPayerGroup:  groupHasPayer,
			})
		}
	}

	return plan, nil
}

func (s *Server) loadMemberRefs(ctx context.Context, ids []int64) (map[int64]models.Member, error) {
	result := make(map[int64]models.Member, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(ctx, `SELECT id, `+userLabelSQL+` AS name, is_active FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("load members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m models.Member
		if err := rows.Scan(&m.ID, &m.Name, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		if !m.IsActive {
			return nil, badRequestf("%s is inactive and cannot be added to a bill", m.Name)
		}
		result[m.ID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load members: %w", err)
	}
	for _, id := range ids {
		if _, ok := result[id]; !ok {
			return nil, badRequestf("member #%d does not exist", id)
		}
	}
	return result, nil
}

func (s *Server) loadResolvedGroups(ctx context.Context, ids []int64) (map[int64]resolvedGroup, error) {
	result := make(map[int64]resolvedGroup, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, name, internal_split_type, is_active
		FROM member_groups WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	for rows.Next() {
		var g resolvedGroup
		var active bool
		if err := rows.Scan(&g.ID, &g.Name, &g.InternalSplitType, &active); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan group: %w", err)
		}
		if !active {
			rows.Close()
			return nil, badRequestf("Group %q is inactive", g.Name)
		}
		g.Members = []resolvedGroupMember{}
		result[g.ID] = g
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	for _, id := range ids {
		if _, ok := result[id]; !ok {
			return nil, badRequestf("group #%d does not exist", id)
		}
	}

	memberRows, err := s.db.Query(ctx, `
		SELECT gm.group_id, gm.member_id, `+userLabelSQLM+`, gm.internal_ratio, m.is_active
		FROM group_members gm
		JOIN users m ON m.id = gm.member_id
		WHERE gm.group_id = ANY($1)
		ORDER BY `+userLabelSQLM+` ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("load group members: %w", err)
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var groupID int64
		var member resolvedGroupMember
		var active bool
		if err := memberRows.Scan(&groupID, &member.MemberID, &member.Name, &member.Ratio, &active); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}
		if !active {
			return nil, badRequestf("%s is inactive; update group %q before using it", member.Name, result[groupID].Name)
		}
		group := result[groupID]
		group.Members = append(group.Members, member)
		result[groupID] = group
	}
	if err := memberRows.Err(); err != nil {
		return nil, fmt.Errorf("load group members: %w", err)
	}
	for _, id := range ids {
		if len(result[id].Members) < 2 {
			return nil, badRequestf("Group %q needs at least two members", result[id].Name)
		}
	}
	return result, nil
}

func (s *Server) storeBill(ctx context.Context, plan *billPlan, attachments []preparedAttachment) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var billID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO bills (title, description, total_amount_cents, paid_by_member_id, split_mode, status, bill_date)
		VALUES ($1, $2, $3, $4, $5, 'OPEN', $6) RETURNING id`,
		plan.Title, plan.Description, plan.TotalCents, plan.PayerID, plan.SplitMode, plan.BillDate,
	).Scan(&billID); err != nil {
		return 0, err
	}

	dir := filepath.Join(s.cfg.UploadDir, strconv.FormatInt(billID, 10))
	if len(attachments) > 0 {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return 0, fmt.Errorf("create upload directory: %w", err)
		}
	}
	keepFiles := false
	defer func() {
		if !keepFiles {
			_ = os.RemoveAll(dir)
		}
	}()

	for _, attachment := range attachments {
		storedName := randomFilename() + attachment.ext
		written, err := writeUpload(attachment.header, filepath.Join(dir, storedName), s.cfg.MaxUploadBytes)
		if err != nil {
			return 0, fmt.Errorf("save %s: %w", attachment.original, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bill_attachments (bill_id, original_filename, stored_filename, mime_type, file_size_bytes)
			VALUES ($1, $2, $3, $4, $5)`,
			billID, attachment.original, storedName, attachment.mimeType, written); err != nil {
			return 0, err
		}
	}

	for _, participant := range plan.Participants {
		if _, err := tx.Exec(ctx, `
			INSERT INTO bill_participants (bill_id, participant_type, member_id, group_id, note, ratio_weight, allocated_amount_cents)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			billID, participant.Type, participant.MemberID, participant.GroupID,
			participant.Note, participant.Weight, participant.Allocated); err != nil {
			return 0, err
		}
	}

	for _, share := range plan.Shares {
		// The payer cannot owe themselves, and a group the payer belongs to is
		// treated as already covered by them.
		settled := share.MemberID == plan.PayerID || share.InPayerGroup
		if _, err := tx.Exec(ctx, `
			INSERT INTO bill_member_shares (bill_id, member_id, source_group_id, owed_amount_cents, is_settled)
			VALUES ($1, $2, $3, $4, $5)`,
			billID, share.MemberID, share.SourceGroupID, share.Owed, settled); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	keepFiles = true
	return billID, nil
}

func (s *Server) prepareAttachments(files []*multipart.FileHeader) ([]preparedAttachment, error) {
	if len(files) > maxAttachments {
		return nil, fmt.Errorf("At most %d attachments are allowed per bill", maxAttachments)
	}
	prepared := make([]preparedAttachment, 0, len(files))
	var totalSize int64

	for _, header := range files {
		original := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
		if original == "" || original == "." {
			original = "attachment"
		}

		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("Could not read %s", original)
		}
		head := make([]byte, 512)
		n, readErr := io.ReadFull(file, head)
		file.Close()
		if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
			return nil, fmt.Errorf("Could not read %s", original)
		}

		mimeType := http.DetectContentType(head[:n])
		ext, allowed := allowedUploads[mimeType]
		if !allowed {
			return nil, fmt.Errorf("%s is not a supported file type (JPEG, PNG, WebP or PDF)", original)
		}

		size := header.Size
		if size < 0 {
			size = 0
		}
		totalSize += size
		if totalSize > s.cfg.MaxUploadBytes {
			return nil, fmt.Errorf("Attachments exceed the %d MB limit", s.cfg.MaxUploadBytes/(1<<20))
		}

		prepared = append(prepared, preparedAttachment{
			header:   header,
			mimeType: mimeType,
			ext:      ext,
			original: original,
			size:     size,
		})
	}
	return prepared, nil
}

func writeUpload(header *multipart.FileHeader, destination string, maxBytes int64) (int64, error) {
	src, err := header.Open()
	if err != nil {
		return 0, err
	}
	defer src.Close()

	dst, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return 0, err
	}

	written, copyErr := io.Copy(dst, io.LimitReader(src, maxBytes+1))
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return 0, closeErr
	}
	if written > maxBytes {
		_ = os.Remove(destination)
		return 0, errors.New("file exceeds the upload limit")
	}
	return written, nil
}

func randomFilename() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}

func (s *Server) handleListBills(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		SELECT b.id, b.title, b.description, b.total_amount_cents, b.paid_by_member_id, `+userLabelSQLM+`,
		       b.split_mode, b.status, b.bill_date, b.created_at,
		       (SELECT COUNT(*) FROM bill_attachments a WHERE a.bill_id = b.id),
		       (SELECT COUNT(*) FROM bill_participants p WHERE p.bill_id = b.id)
		FROM bills b
		JOIN users m ON m.id = b.paid_by_member_id
		ORDER BY b.bill_date DESC, b.id DESC`)
	if err != nil {
		s.logger.Error("list bills", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load bills")
		return
	}
	defer rows.Close()

	bills := make([]models.BillSummary, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var b models.BillSummary
		var billDate time.Time
		if err := rows.Scan(&b.ID, &b.Title, &b.Description, &b.TotalAmountCents,
			&b.PaidByMemberID, &b.PaidByName, &b.SplitMode, &b.Status, &billDate, &b.CreatedAt,
			&b.AttachmentCount, &b.ParticipantCount); err != nil {
			s.logger.Error("scan bill", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not load bills")
			return
		}
		b.BillDate = billDate.Format("2006-01-02")
		b.Shares = []models.SettlementShare{}
		bills = append(bills, b)
		ids = append(ids, b.ID)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load bills")
		return
	}

	if len(ids) > 0 {
		shareRows, err := s.db.Query(r.Context(), `
			SELECT s.bill_id, s.id, s.member_id, `+userLabelSQLM+`, s.owed_amount_cents,
			       s.is_settled, s.settle_requested_at IS NOT NULL,
			       (b.paid_by_member_id = s.member_id) AS is_payer
			FROM bill_member_shares s
			JOIN users m ON m.id = s.member_id
			JOIN bills b ON b.id = s.bill_id
			WHERE s.bill_id = ANY($1)
			ORDER BY s.id`, ids)
		if err != nil {
			s.logger.Error("list bill shares", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not load bills")
			return
		}
		defer shareRows.Close()

		byBill := make(map[int64][]models.SettlementShare, len(ids))
		for shareRows.Next() {
			var billID int64
			var sh models.SettlementShare
			if err := shareRows.Scan(&billID, &sh.ShareID, &sh.MemberID, &sh.MemberName,
				&sh.OwedAmountCents, &sh.IsSettled, &sh.Requested, &sh.IsPayer); err != nil {
				writeError(w, http.StatusInternalServerError, "Could not load bills")
				return
			}
			byBill[billID] = append(byBill[billID], sh)
		}
		if err := shareRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load bills")
			return
		}
		for i := range bills {
			if shares, ok := byBill[bills[i].ID]; ok {
				bills[i].Shares = shares
			}
		}
	}

	writeJSON(w, http.StatusOK, bills)
}

func (s *Server) handlePublicListBills(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.db.Query(ctx, `
		SELECT b.id, b.title, b.description, b.total_amount_cents, `+userLabelSQLM+`,
		       b.split_mode, b.status, b.bill_date,
		       (SELECT COUNT(*) FROM bill_attachments a WHERE a.bill_id = b.id),
		       (SELECT COUNT(*) FROM bill_participants p WHERE p.bill_id = b.id)
		FROM bills b
		JOIN users m ON m.id = b.paid_by_member_id
		ORDER BY b.bill_date DESC, b.id DESC`)
	if err != nil {
		s.logger.Error("list public bills", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load bills")
		return
	}
	defer rows.Close()

	bills := make([]models.PublicBillSummary, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var b models.PublicBillSummary
		var billDate time.Time
		if err := rows.Scan(&b.ID, &b.Title, &b.Description, &b.TotalAmountCents, &b.PaidByName,
			&b.SplitMode, &b.Status, &billDate, &b.AttachmentCount, &b.ParticipantCount); err != nil {
			s.logger.Error("scan public bill", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not load bills")
			return
		}
		b.BillDate = billDate.Format("2006-01-02")
		b.Members = []string{}
		bills = append(bills, b)
		ids = append(ids, b.ID)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load bills")
		return
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusOK, bills)
		return
	}

	memberRows, err := s.db.Query(ctx, `
		SELECT s.bill_id, `+userLabelSQLM+`
		FROM bill_member_shares s
		JOIN users m ON m.id = s.member_id
		WHERE s.bill_id = ANY($1)
		ORDER BY `+userLabelSQLM+` ASC`, ids)
	if err != nil {
		s.logger.Error("list public bill members", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load bills")
		return
	}
	defer memberRows.Close()

	membersByBill := make(map[int64][]string, len(ids))
	for memberRows.Next() {
		var billID int64
		var name string
		if err := memberRows.Scan(&billID, &name); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load bills")
			return
		}
		membersByBill[billID] = append(membersByBill[billID], name)
	}
	if err := memberRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load bills")
		return
	}

	for i := range bills {
		if names := membersByBill[bills[i].ID]; names != nil {
			bills[i].Members = names
		}
	}
	writeJSON(w, http.StatusOK, bills)
}

func (s *Server) handlePublicGetBill(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	bill, err := s.loadBill(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Bill not found")
		return
	}
	if err != nil {
		s.logger.Error("load public bill", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load bill")
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (s *Server) handleUpdateBillStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := strings.ToUpper(strings.TrimSpace(body.Status))
	if status != "OPEN" && status != "SETTLED" {
		writeError(w, http.StatusBadRequest, "Status must be OPEN or SETTLED")
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.Error("update bill status", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update bill")
		return
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `UPDATE bills SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		s.logger.Error("update bill status", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update bill")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Bill not found")
		return
	}

	// Keep shares in step with the status: settling the bill settles every share
	// (clearing it from balances and "To settle"), reopening reopens the debts. The
	// payer's own share stays settled — they already covered it.
	var shareErr error
	if status == "SETTLED" {
		_, shareErr = tx.Exec(ctx, `
			UPDATE bill_member_shares SET is_settled = TRUE, settle_requested_at = NULL
			WHERE bill_id = $1`, id)
	} else {
		_, shareErr = tx.Exec(ctx, `
			UPDATE bill_member_shares s SET is_settled = FALSE, settle_requested_at = NULL
			FROM bills b
			WHERE b.id = s.bill_id AND s.bill_id = $1
			  AND s.member_id <> b.paid_by_member_id
			  AND NOT (s.source_group_id IS NOT NULL AND EXISTS (
			      SELECT 1 FROM group_members gm
			      WHERE gm.group_id = s.source_group_id AND gm.member_id = b.paid_by_member_id
			  ))`, id)
	}
	if shareErr != nil {
		s.logger.Error("sync bill shares", "error", shareErr)
		writeError(w, http.StatusInternalServerError, "Could not update bill")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("commit bill status", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update bill")
		return
	}

	// Marking the whole bill settled (or reopening it) moves every share, so every
	// participant needs to hear about it: the payer's counterparties get the alert,
	// and the rest of the bill just re-renders.
	actorID, _ := s.auth.RequestUser(r)
	info := s.billEventInfo(ctx, id)
	info.ActorID = actorID
	info.ActorName = s.displayName(ctx, actorID)

	recipients, err := s.billParticipantIDs(ctx, id)
	if err != nil {
		s.logger.Error("bill recipients", "bill_id", id, "error", err)
	} else {
		kind := realtime.TypeBillUpdated
		if status == "SETTLED" {
			kind = realtime.TypeBillSettled
		}
		s.emitBill(excludingID(recipients, actorID), kind, info)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

// syncBillStatus recomputes a bill's status from its shares: once every share is
// settled the bill is SETTLED, and any outstanding share reopens it.
func (s *Server) syncBillStatus(ctx context.Context, billID int64) (string, error) {
	var total, settled int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE is_settled)
		FROM bill_member_shares WHERE bill_id = $1`, billID).Scan(&total, &settled); err != nil {
		return "", fmt.Errorf("count shares: %w", err)
	}

	status := "OPEN"
	if total > 0 && settled == total {
		status = "SETTLED"
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE bills SET status = $1 WHERE id = $2 AND status <> $1`, status, billID); err != nil {
		return "", fmt.Errorf("update bill status: %w", err)
	}
	return status, nil
}

// handleUpdateShareSettled records settlement state for one share. Two actors are
// involved: the debtor may "request" that they have paid (action=request), and the
// bill's payer approves or rejects it (action=settle / action=reject).
func (s *Server) handleUpdateShareSettled(w http.ResponseWriter, r *http.Request) {
	billID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	shareID, ok := pathID(w, r, "shareID")
	if !ok {
		return
	}

	var body struct {
		Action  string `json:"action"`
		Settled *bool  `json:"settled"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	viewerID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	ctx := r.Context()
	var payerID, debtorID int64
	var settled bool
	err := s.db.QueryRow(ctx, `
		SELECT b.paid_by_member_id, s.member_id, s.is_settled
		FROM bill_member_shares s
		JOIN bills b ON b.id = s.bill_id
		WHERE s.id = $1 AND s.bill_id = $2`, shareID, billID).Scan(&payerID, &debtorID, &settled)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Share not found")
		return
	}
	if err != nil {
		s.logger.Error("load share", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not update share")
		return
	}

	isAdmin := s.requestIsAdmin(r)
	isPayer := payerID == viewerID
	isDebtor := debtorID == viewerID

	action := strings.ToLower(strings.TrimSpace(body.Action))
	if action == "" {
		// Back-compat: a bare settled boolean is the payer approving or reopening.
		if body.Settled == nil {
			writeError(w, http.StatusBadRequest, "action is required")
			return
		}
		if *body.Settled {
			action = "settle"
		} else {
			action = "reopen"
		}
	}

	switch action {
	case "request":
		// The debtor says they have paid; the payer still has to confirm.
		if !isDebtor && !isAdmin {
			writeError(w, http.StatusForbidden, "Only the member who owes this share can request settlement")
			return
		}
		if settled {
			writeJSON(w, http.StatusOK, map[string]any{"is_settled": true, "requested": false})
			return
		}
		if _, err := s.db.Exec(ctx, `
			UPDATE bill_member_shares SET settle_requested_at = now()
			WHERE id = $1`, shareID); err != nil {
			s.logger.Error("request settlement", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not request settlement")
			return
		}
		s.notifyShareChange(ctx, billID, payerID, "request", viewerID)
		writeJSON(w, http.StatusOK, map[string]any{"is_settled": false, "requested": true})

	case "settle", "reject", "reopen":
		// Confirmation is the payer's call.
		if !isPayer && !isAdmin {
			writeError(w, http.StatusForbidden, "Only the payer can confirm a settlement")
			return
		}
		next := action == "settle"
		// Confirming or rejecting both clear the pending request.
		if _, err := s.db.Exec(ctx, `
			UPDATE bill_member_shares
			SET is_settled = $1, settle_requested_at = NULL
			WHERE id = $2`, next, shareID); err != nil {
			s.logger.Error("update share settled", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not update share")
			return
		}
		// The bill is only complete once every share is settled.
		billStatus, err := s.syncBillStatus(ctx, billID)
		if err != nil {
			s.logger.Error("sync bill status", "error", err)
		}
		s.notifyShareChange(ctx, billID, debtorID, action, viewerID)
		writeJSON(w, http.StatusOK, map[string]any{"is_settled": next, "requested": false, "bill_status": billStatus})

	default:
		writeError(w, http.StatusBadRequest, "action must be request, settle, reject or reopen")
	}
}

func (s *Server) handleGetBill(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	bill, err := s.loadBill(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Bill not found")
		return
	}
	if err != nil {
		s.logger.Error("load bill", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load bill")
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (s *Server) loadBill(ctx context.Context, id int64) (models.Bill, error) {
	var bill models.Bill
	var billDate time.Time
	err := s.db.QueryRow(ctx, `
		SELECT b.id, b.title, b.description, b.total_amount_cents, b.paid_by_member_id, `+userLabelSQLM+`,
		       b.split_mode, b.status, b.bill_date, b.created_at,
		       (SELECT COUNT(*) FROM bill_attachments a WHERE a.bill_id = b.id),
		       (SELECT COUNT(*) FROM bill_participants p WHERE p.bill_id = b.id)
		FROM bills b
		JOIN users m ON m.id = b.paid_by_member_id
		WHERE b.id = $1`, id,
	).Scan(&bill.ID, &bill.Title, &bill.Description, &bill.TotalAmountCents,
		&bill.PaidByMemberID, &bill.PaidByName, &bill.SplitMode, &bill.Status, &billDate, &bill.CreatedAt,
		&bill.AttachmentCount, &bill.ParticipantCount)
	if err != nil {
		return models.Bill{}, err
	}
	bill.BillDate = billDate.Format("2006-01-02")
	bill.Participants = []models.BillParticipant{}
	bill.MemberShares = []models.BillMemberShare{}
	bill.Attachments = []models.Attachment{}

	participantRows, err := s.db.Query(ctx, `
		SELECT p.id, p.participant_type, p.member_id, p.group_id,
		       COALESCE(`+userLabelSQLM+`, g.name, ''), p.note, p.ratio_weight, p.allocated_amount_cents
		FROM bill_participants p
		LEFT JOIN users m ON m.id = p.member_id
		LEFT JOIN member_groups g ON g.id = p.group_id
		WHERE p.bill_id = $1
		ORDER BY p.id`, id)
	if err != nil {
		return models.Bill{}, err
	}
	for participantRows.Next() {
		var p models.BillParticipant
		var memberID, groupID *int64
		if err := participantRows.Scan(&p.ID, &p.ParticipantType, &memberID, &groupID,
			&p.Name, &p.Note, &p.RatioWeight, &p.AllocatedAmountCents); err != nil {
			participantRows.Close()
			return models.Bill{}, err
		}
		p.MemberID = memberID
		p.GroupID = groupID
		bill.Participants = append(bill.Participants, p)
	}
	participantRows.Close()
	if err := participantRows.Err(); err != nil {
		return models.Bill{}, err
	}

	shareRows, err := s.db.Query(ctx, `
		SELECT s.id, s.member_id, `+userLabelSQLM+`, s.source_group_id, s.owed_amount_cents,
		       s.is_settled, s.settle_requested_at
		FROM bill_member_shares s
		JOIN users m ON m.id = s.member_id
		WHERE s.bill_id = $1
		ORDER BY s.id`, id)
	if err != nil {
		return models.Bill{}, err
	}
	for shareRows.Next() {
		var share models.BillMemberShare
		var sourceGroupID *int64
		var requestedAt *time.Time
		if err := shareRows.Scan(&share.ID, &share.MemberID, &share.MemberName, &sourceGroupID,
			&share.OwedAmountCents, &share.IsSettled, &requestedAt); err != nil {
			shareRows.Close()
			return models.Bill{}, err
		}
		share.SourceGroupID = sourceGroupID
		share.SettleRequested = requestedAt
		bill.MemberShares = append(bill.MemberShares, share)
	}
	shareRows.Close()
	if err := shareRows.Err(); err != nil {
		return models.Bill{}, err
	}

	attachmentRows, err := s.db.Query(ctx, `
		SELECT id, original_filename, stored_filename, mime_type, file_size_bytes, uploaded_at
		FROM bill_attachments
		WHERE bill_id = $1
		ORDER BY id`, id)
	if err != nil {
		return models.Bill{}, err
	}
	defer attachmentRows.Close()
	for attachmentRows.Next() {
		var attachment models.Attachment
		var storedName string
		if err := attachmentRows.Scan(&attachment.ID, &attachment.OriginalFilename, &storedName,
			&attachment.MimeType, &attachment.FileSizeBytes, &attachment.UploadedAt); err != nil {
			return models.Bill{}, err
		}
		attachment.URL = fmt.Sprintf("/uploads/%d/%s", id, storedName)
		bill.Attachments = append(bill.Attachments, attachment)
	}
	if err := attachmentRows.Err(); err != nil {
		return models.Bill{}, err
	}

	return bill, nil
}

func (s *Server) handleDeleteBill(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	ctx := r.Context()
	tag, err := s.db.Exec(ctx, `DELETE FROM bills WHERE id = $1`, id)
	if err != nil {
		s.logger.Error("delete bill", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not delete bill")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Bill not found")
		return
	}

	if err := os.RemoveAll(filepath.Join(s.cfg.UploadDir, strconv.FormatInt(id, 10))); err != nil {
		s.logger.Warn("remove bill uploads", "bill_id", id, "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSettlements lists the shares that concern the caller (money they still owe,
// or shares awaiting their confirmation as the payer) and attaches each bill's full
// participant breakdown so the dashboard can expand a row.
func (s *Server) handleSettlements(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := s.auth.RequestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	ctx := r.Context()
	rows, err := s.db.Query(ctx, `
		SELECT s.id, b.id, b.title, b.bill_date, b.status, s.owed_amount_cents, s.is_settled,
		       s.settle_requested_at IS NOT NULL,
		       CASE WHEN s.member_id = $1 THEN 'debtor' ELSE 'payer' END AS role,
		       `+userLabelSQLM+` AS debtor_name,
		       COALESCE((
		           SELECT p.note FROM bill_participants p
		           WHERE p.bill_id = b.id AND p.participant_type = 'MEMBER' AND p.member_id = s.member_id
		           LIMIT 1
		       ), '') AS note
		FROM bill_member_shares s
		JOIN bills b ON b.id = s.bill_id
		JOIN users m ON m.id = s.member_id
		WHERE ((s.member_id = $1 AND s.is_settled = FALSE)
		   OR (b.paid_by_member_id = $1 AND s.member_id <> $1 AND s.is_settled = FALSE))
		  AND b.status <> 'SETTLED'
		ORDER BY b.bill_date DESC, b.id DESC`, viewerID)
	if err != nil {
		s.logger.Error("list settlements", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load settlements")
		return
	}
	defer rows.Close()

	items := make([]models.SettlementItem, 0)
	billIDs := make([]int64, 0)
	for rows.Next() {
		var it models.SettlementItem
		var billDate time.Time
		if err := rows.Scan(&it.ShareID, &it.BillID, &it.BillTitle, &billDate, &it.BillStatus,
			&it.OwedAmountCents, &it.IsSettled, &it.Requested, &it.Role, &it.CounterpartyName, &it.Note); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load settlements")
			return
		}
		it.BillDate = billDate.Format("2006-01-02")
		it.Shares = []models.SettlementShare{}
		items = append(items, it)
		billIDs = append(billIDs, it.BillID)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load settlements")
		return
	}

	if len(billIDs) > 0 {
		// One query for every participant across the listed bills.
		shareRows, err := s.db.Query(ctx, `
			SELECT s.bill_id, s.id, s.member_id, `+userLabelSQLM+`, s.owed_amount_cents,
			       s.is_settled, s.settle_requested_at IS NOT NULL,
			       (b.paid_by_member_id = s.member_id) AS is_payer
			FROM bill_member_shares s
			JOIN users m ON m.id = s.member_id
			JOIN bills b ON b.id = s.bill_id
			WHERE s.bill_id = ANY($1)
			ORDER BY s.id`, billIDs)
		if err != nil {
			s.logger.Error("list settlement shares", "error", err)
			writeError(w, http.StatusInternalServerError, "Could not load settlements")
			return
		}
		defer shareRows.Close()

		byBill := make(map[int64][]models.SettlementShare, len(billIDs))
		for shareRows.Next() {
			var billID int64
			var sh models.SettlementShare
			if err := shareRows.Scan(&billID, &sh.ShareID, &sh.MemberID, &sh.MemberName,
				&sh.OwedAmountCents, &sh.IsSettled, &sh.Requested, &sh.IsPayer); err != nil {
				writeError(w, http.StatusInternalServerError, "Could not load settlements")
				return
			}
			byBill[billID] = append(byBill[billID], sh)
		}
		if err := shareRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load settlements")
			return
		}
		for i := range items {
			if shares, ok := byBill[items[i].BillID]; ok {
				items[i].Shares = shares
			}
		}
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request) {

	// The caller's own row is flagged rather than dropped: the dashboard hides it
	// from the list but still needs it for the summary counts.
	viewerID, _ := s.auth.RequestUser(r)

	// Only outstanding shares of open bills move money. A member's net is what others
	// still owe them minus what they still owe. A settled bill owes nothing even if
	// an old build left its shares flagged open, so the bill status is checked
	// alongside the per-share flag rather than trusting the flag alone.
	rows, err := s.db.Query(r.Context(), `
		SELECT m.id, `+userLabelSQLM+`, m.is_active, (m.id = $1) AS is_self,
		       COALESCE(credit.total, 0)::bigint,
		       COALESCE(debt.total, 0)::bigint,
		       (COALESCE(credit.total, 0) - COALESCE(debt.total, 0))::bigint
		FROM users m
		LEFT JOIN (
			SELECT b.paid_by_member_id AS member_id, SUM(s.owed_amount_cents) AS total
			FROM bill_member_shares s
			JOIN bills b ON b.id = s.bill_id
			WHERE s.is_settled = FALSE AND b.status <> 'SETTLED'
			  AND s.member_id <> b.paid_by_member_id
			GROUP BY b.paid_by_member_id
		) credit ON credit.member_id = m.id
		LEFT JOIN (
			SELECT s.member_id, SUM(s.owed_amount_cents) AS total
			FROM bill_member_shares s
			JOIN bills b ON b.id = s.bill_id
			WHERE s.is_settled = FALSE AND b.status <> 'SETTLED'
			  AND s.member_id <> b.paid_by_member_id
			GROUP BY s.member_id
		) debt ON debt.member_id = m.id
		ORDER BY `+userLabelSQLM+` ASC`, viewerID)
	if err != nil {
		s.logger.Error("load balances", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load balances")
		return
	}
	defer rows.Close()

	balances := make([]models.Balance, 0)
	for rows.Next() {
		var b models.Balance
		if err := rows.Scan(&b.MemberID, &b.Name, &b.IsActive, &b.IsSelf,
			&b.PaidCents, &b.OwedCents, &b.NetCents); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load balances")
			return
		}
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load balances")
		return
	}
	writeJSON(w, http.StatusOK, balances)
}

package models

import "time"

type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Phone       string    `json:"phone"`
	Notes       string    `json:"notes"`
	AvatarColor string    `json:"avatar_color"`
	IsActive    bool      `json:"is_active"`
	HasPassword bool      `json:"has_password"`
	IsPublic    bool      `json:"is_public"`
	Language    string    `json:"language"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MemberGroupRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Member struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Phone       string           `json:"phone"`
	Email       string           `json:"email"`
	Notes       string           `json:"notes"`
	AvatarColor string           `json:"avatar_color"`
	IsActive    bool             `json:"is_active"`
	HasPassword bool             `json:"has_password"`
	IsPublic    bool             `json:"is_public"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Groups      []MemberGroupRef `json:"groups"`
}

type GroupMember struct {
	MemberID      int64   `json:"member_id"`
	Name          string  `json:"name"`
	InternalRatio float64 `json:"internal_ratio"`
}

type Group struct {
	ID                int64         `json:"id"`
	Name              string        `json:"name"`
	InternalSplitType string        `json:"internal_split_type"`
	IsActive          bool          `json:"is_active"`
	LeaderUserID      *int64        `json:"leader_user_id,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	Members           []GroupMember `json:"members"`
}

type BillSummary struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	TotalAmountCents int64     `json:"total_amount_cents"`
	PaidByMemberID   int64     `json:"paid_by_member_id"`
	PaidByName       string    `json:"paid_by_name"`
	SplitMode        string    `json:"split_mode"`
	Status           string    `json:"status"`
	BillDate         string    `json:"bill_date"`
	CreatedAt        time.Time `json:"created_at"`
	AttachmentCount  int       `json:"attachment_count"`
	ParticipantCount int       `json:"participant_count"`
}

type PublicBillSummary struct {
	ID               int64    `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	TotalAmountCents int64    `json:"total_amount_cents"`
	PaidByName       string   `json:"paid_by_name"`
	SplitMode        string   `json:"split_mode"`
	Status           string   `json:"status"`
	BillDate         string   `json:"bill_date"`
	AttachmentCount  int      `json:"attachment_count"`
	ParticipantCount int      `json:"participant_count"`
	Members          []string `json:"members"`
}

type BillParticipant struct {
	ID                   int64   `json:"id"`
	ParticipantType      string  `json:"participant_type"`
	MemberID             *int64  `json:"member_id,omitempty"`
	GroupID              *int64  `json:"group_id,omitempty"`
	Name                 string  `json:"name"`
	Note                 string  `json:"note"`
	RatioWeight          float64 `json:"ratio_weight"`
	AllocatedAmountCents int64   `json:"allocated_amount_cents"`
}

type BillMemberShare struct {
	ID              int64      `json:"id"`
	MemberID        int64      `json:"member_id"`
	MemberName      string     `json:"member_name"`
	SourceGroupID   *int64     `json:"source_group_id,omitempty"`
	OwedAmountCents int64      `json:"owed_amount_cents"`
	IsSettled       bool       `json:"is_settled"`
	SettleRequested *time.Time `json:"settle_requested_at,omitempty"`
}

// SettlementItem is one row in the dashboard's "to settle" list.
type SettlementItem struct {
	ShareID          int64  `json:"share_id"`
	BillID           int64  `json:"bill_id"`
	BillTitle        string `json:"bill_title"`
	BillDate         string `json:"bill_date"`
	OwedAmountCents  int64  `json:"owed_amount_cents"`
	Role             string `json:"role"` // "debtor" or "payer"
	CounterpartyName string `json:"counterparty_name"`
	Note             string `json:"note"`
	IsSettled        bool   `json:"is_settled"`
	Requested        bool   `json:"requested"`
}

type Attachment struct {
	ID               int64     `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	FileSizeBytes    int64     `json:"file_size_bytes"`
	UploadedAt       time.Time `json:"uploaded_at"`
	URL              string    `json:"url"`
}

type Bill struct {
	BillSummary
	Participants []BillParticipant `json:"participants"`
	MemberShares []BillMemberShare `json:"member_shares"`
	Attachments  []Attachment      `json:"attachments"`
}

type Balance struct {
	MemberID  int64  `json:"member_id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	IsSelf    bool   `json:"is_self"`
	PaidCents int64  `json:"paid_cents"`
	OwedCents int64  `json:"owed_cents"`
	NetCents  int64  `json:"net_cents"`
}

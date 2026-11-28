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

// Project is the tenant boundary: one bill book shared by a set of people.
type Project struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	OwnerUserID int64  `json:"owner_user_id"`
	// Role is the caller's role in this project: "owner" or "member".
	Role        string `json:"role,omitempty"`
	MemberCount int    `json:"member_count"`
	IsActive    bool   `json:"is_active"`
	// PublicSlug is only returned to the owner, since it is the share credential.
	PublicSlug    string    `json:"public_slug,omitempty"`
	PublicEnabled bool      `json:"public_enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ProjectMember is one person in a project, for the membership screen.
type ProjectMember struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
	// HasPassword is false for people who were added but never activated.
	HasPassword bool `json:"has_password"`
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
	// ProjectName/Phone/Notes are the raw per-project overrides. An empty value
	// means "inherit the account profile" and the Name/Phone/Notes above show
	// the resolved value.
	ProjectName  string `json:"project_name"`
	ProjectPhone string `json:"project_phone"`
	ProjectNotes string `json:"project_notes"`
	// ProfileName/Phone/Notes are the account's global values, surfaced so the
	// edit form can hint what an empty project field will fall back to.
	ProfileName  string           `json:"profile_name"`
	ProfilePhone string           `json:"profile_phone"`
	ProfileNotes string           `json:"profile_notes"`
	// InviteSent is set only on a create response, to tell the caller whether an
	// activation/added email was dispatched.
	InviteSent bool             `json:"invite_sent,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Groups     []MemberGroupRef `json:"groups"`
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
	// Shares lets the dashboard expand a row without a second request.
	Shares []SettlementShare `json:"shares"`
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
	BillStatus       string `json:"bill_status"`
	OwedAmountCents  int64  `json:"owed_amount_cents"`
	Role             string `json:"role"` // "debtor" or "payer"
	CounterpartyName string `json:"counterparty_name"`
	Note             string `json:"note"`
	IsSettled        bool   `json:"is_settled"`
	Requested        bool   `json:"requested"`
	// Shares carries every participant on the bill so the UI can show a breakdown.
	Shares []SettlementShare `json:"shares"`
}

// SettlementShare is one participant row inside a settlement's breakdown.
type SettlementShare struct {
	ShareID         int64  `json:"share_id"`
	MemberID        int64  `json:"member_id"`
	MemberName      string `json:"member_name"`
	OwedAmountCents int64  `json:"owed_amount_cents"`
	IsSettled       bool   `json:"is_settled"`
	Requested       bool   `json:"requested"`
	IsPayer         bool   `json:"is_payer"`
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

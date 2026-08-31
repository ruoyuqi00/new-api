package model

import (
	"fmt"
	"strings"
)

const (
	TicketCategoryGeneral = "general"
	TicketCategoryRefund  = "refund"

	TicketStatusOpen         = "open"
	TicketStatusPendingUser  = "pending_user"
	TicketStatusPendingAdmin = "pending_admin"
	TicketStatusClosed       = "closed"

	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"

	TicketAuthorRoleUser  = "user"
	TicketAuthorRoleAdmin = "admin"

	maxTicketSubjectLength = 255
	maxTicketBodyLength    = 32768
)

type Ticket struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	UserID         int    `json:"user_id" gorm:"index"`
	Subject        string `json:"subject" gorm:"type:varchar(255)"`
	Category       string `json:"category" gorm:"type:varchar(24);index"`
	Status         string `json:"status" gorm:"type:varchar(24);index"`
	Priority       string `json:"priority" gorm:"type:varchar(24);index"`
	MessageCount   int    `json:"message_count"`
	UnreadForUser  int    `json:"unread_for_user"`
	UnreadForAdmin int    `json:"unread_for_admin"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
	LastMessageAt  int64  `json:"last_message_at" gorm:"bigint;index"`
}

type TicketMessage struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	TicketID   int64  `json:"ticket_id" gorm:"index"`
	AuthorID   int    `json:"author_id" gorm:"index"`
	AuthorRole string `json:"author_role" gorm:"type:varchar(16);index"`
	Body       string `json:"body" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

type TicketAttachment struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	TicketID    int64  `json:"ticket_id" gorm:"index"`
	MessageID   int64  `json:"message_id" gorm:"index"`
	UploaderID  int    `json:"uploader_id" gorm:"index"`
	StorageKey  string `json:"-" gorm:"type:varchar(255)"`
	DisplayName string `json:"display_name" gorm:"type:varchar(255)"`
	MIMEType    string `json:"mime_type" gorm:"type:varchar(128)"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;index"`
}

func (Ticket) TableName() string {
	return "tickets"
}

func (TicketMessage) TableName() string {
	return "ticket_messages"
}

func (TicketAttachment) TableName() string {
	return "ticket_attachments"
}

func (ticket Ticket) Validate() error {
	if strings.TrimSpace(ticket.Subject) == "" {
		return fmt.Errorf("ticket subject is required")
	}
	if len([]rune(ticket.Subject)) > maxTicketSubjectLength {
		return fmt.Errorf("ticket subject exceeds %d characters", maxTicketSubjectLength)
	}
	if !validTicketCategory(ticket.Category) {
		return fmt.Errorf("invalid ticket category %q", ticket.Category)
	}
	if !validTicketStatus(ticket.Status) && ticket.Status != "" {
		return fmt.Errorf("invalid ticket status %q", ticket.Status)
	}
	if !validTicketPriority(ticket.Priority) {
		return fmt.Errorf("invalid ticket priority %q", ticket.Priority)
	}
	return nil
}

func (message TicketMessage) Validate() error {
	if message.AuthorID <= 0 {
		return fmt.Errorf("ticket message author is required")
	}
	if message.AuthorRole != TicketAuthorRoleUser && message.AuthorRole != TicketAuthorRoleAdmin {
		return fmt.Errorf("invalid ticket message author role %q", message.AuthorRole)
	}
	if strings.TrimSpace(message.Body) == "" {
		return fmt.Errorf("ticket message body is required")
	}
	if len([]rune(message.Body)) > maxTicketBodyLength {
		return fmt.Errorf("ticket message body exceeds %d characters", maxTicketBodyLength)
	}
	return nil
}

func validTicketCategory(value string) bool {
	return value == TicketCategoryGeneral || value == TicketCategoryRefund
}

func validTicketStatus(value string) bool {
	switch value {
	case TicketStatusOpen, TicketStatusPendingUser, TicketStatusPendingAdmin, TicketStatusClosed:
		return true
	default:
		return false
	}
}

func validTicketPriority(value string) bool {
	switch value {
	case TicketPriorityNormal, TicketPriorityHigh, TicketPriorityUrgent:
		return true
	default:
		return false
	}
}

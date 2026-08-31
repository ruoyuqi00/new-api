package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrTicketNotFound = errors.New("ticket not found")
	ErrTicketClosed   = errors.New("ticket is closed")
)

type CreateTicketInput struct {
	Subject  string
	Category string
	Priority string
	Body     string
}

type TicketListFilter struct {
	Status   string
	Category string
	Priority string
	Keyword  string
	Page     int
	PageSize int
}

type TicketDetail struct {
	Ticket      model.Ticket
	Messages    []model.TicketMessage
	Attachments []model.TicketAttachment
}

func CreateTicket(ctx context.Context, userID int, input CreateTicketInput) (*model.Ticket, error) {
	if userID <= 0 {
		return nil, errors.New("ticket user is required")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = model.TicketCategoryGeneral
	}
	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = model.TicketPriorityNormal
	}
	now := common.GetTimestamp()
	ticket := &model.Ticket{
		UserID:         userID,
		Subject:        strings.TrimSpace(input.Subject),
		Category:       category,
		Status:         model.TicketStatusOpen,
		Priority:       priority,
		MessageCount:   1,
		UnreadForAdmin: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastMessageAt:  now,
	}
	message := &model.TicketMessage{
		AuthorID:   userID,
		AuthorRole: model.TicketAuthorRoleUser,
		Body:       strings.TrimSpace(input.Body),
		CreatedAt:  now,
	}
	if err := ticket.Validate(); err != nil {
		return nil, err
	}
	if err := message.Validate(); err != nil {
		return nil, err
	}
	if err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		message.TicketID = ticket.ID
		return tx.Create(message).Error
	}); err != nil {
		return nil, err
	}
	return ticket, nil
}

func ListTickets(ctx context.Context, userID int, isAdmin bool, filter TicketListFilter) ([]model.Ticket, int64, error) {
	if !isAdmin && userID <= 0 {
		return nil, 0, ErrTicketNotFound
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := model.DB.WithContext(ctx).Model(&model.Ticket{})
	if !isAdmin {
		query = query.Where("user_id = ?", userID)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.Category); value != "" {
		query = query.Where("category = ?", value)
	}
	if value := strings.TrimSpace(filter.Priority); value != "" {
		query = query.Where("priority = ?", value)
	}
	if value := strings.TrimSpace(filter.Keyword); value != "" {
		query = query.Where("subject LIKE ?", "%"+value+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.Ticket, 0)
	err := query.Order("last_message_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func GetTicketDetail(ctx context.Context, ticketID int64, userID int, isAdmin bool) (*TicketDetail, error) {
	if ticketID <= 0 {
		return nil, ErrTicketNotFound
	}
	var detail TicketDetail
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("id = ?", ticketID)
		if !isAdmin {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&detail.Ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTicketNotFound
			}
			return err
		}
		if err := tx.Where("ticket_id = ?", ticketID).Order("created_at ASC, id ASC").Find(&detail.Messages).Error; err != nil {
			return err
		}
		if err := tx.Where("ticket_id = ?", ticketID).Order("id ASC").Find(&detail.Attachments).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{}
		if isAdmin {
			updates["unread_for_admin"] = 0
		} else {
			updates["unread_for_user"] = 0
		}
		return tx.Model(&model.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func AddTicketMessage(ctx context.Context, ticketID int64, authorID int, authorRole string, body string) (*model.TicketMessage, error) {
	message := &model.TicketMessage{TicketID: ticketID, AuthorID: authorID, AuthorRole: authorRole, Body: strings.TrimSpace(body), CreatedAt: common.GetTimestamp()}
	if err := message.Validate(); err != nil {
		return nil, err
	}
	if authorRole != model.TicketAuthorRoleAdmin && authorRole != model.TicketAuthorRoleUser {
		return nil, fmt.Errorf("invalid ticket author role")
	}
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ticket model.Ticket
		query := tx.Where("id = ?", ticketID)
		if authorRole == model.TicketAuthorRoleUser {
			query = query.Where("user_id = ?", authorID)
		}
		if err := query.First(&ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTicketNotFound
			}
			return err
		}
		if ticket.Status == model.TicketStatusClosed && authorRole == model.TicketAuthorRoleUser {
			return ErrTicketClosed
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		status := model.TicketStatusPendingAdmin
		unreadForUser := ticket.UnreadForUser
		unreadForAdmin := ticket.UnreadForAdmin
		if authorRole == model.TicketAuthorRoleAdmin {
			status = model.TicketStatusPendingUser
			unreadForUser++
		} else {
			unreadForAdmin++
		}
		return tx.Model(&model.Ticket{}).Where("id = ?", ticketID).Updates(map[string]interface{}{
			"status":           status,
			"message_count":    gorm.Expr("message_count + ?", 1),
			"unread_for_user":  unreadForUser,
			"unread_for_admin": unreadForAdmin,
			"last_message_at":  message.CreatedAt,
			"updated_at":       message.CreatedAt,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return message, nil
}

func UpdateTicketState(ctx context.Context, ticketID int64, actorRole string, status *string, priority *string) (*model.Ticket, error) {
	if actorRole != model.TicketAuthorRoleAdmin {
		return nil, errors.New("only administrators can update ticket state")
	}
	if status == nil && priority == nil {
		return nil, errors.New("ticket update is empty")
	}
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if status != nil {
		if !validTicketStatusValue(*status) {
			return nil, fmt.Errorf("invalid ticket status %q", *status)
		}
		updates["status"] = *status
	}
	if priority != nil {
		if !validTicketPriorityValue(*priority) {
			return nil, fmt.Errorf("invalid ticket priority %q", *priority)
		}
		updates["priority"] = *priority
	}
	var ticket model.Ticket
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTicketNotFound
			}
			return err
		}
		if err := tx.Model(&model.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", ticketID).First(&ticket).Error
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func validTicketStatusValue(value string) bool {
	switch value {
	case model.TicketStatusOpen, model.TicketStatusPendingUser, model.TicketStatusPendingAdmin, model.TicketStatusClosed:
		return true
	default:
		return false
	}
}

func validTicketPriorityValue(value string) bool {
	switch value {
	case model.TicketPriorityNormal, model.TicketPriorityHigh, model.TicketPriorityUrgent:
		return true
	default:
		return false
	}
}

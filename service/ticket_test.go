package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTicketServiceTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:ticket_service_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Ticket{}, &model.TicketMessage{}, &model.TicketAttachment{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
}

func TestCreateTicketCreatesFirstMessageAtomically(t *testing.T) {
	setupTicketServiceTestDB(t)
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{
		Subject:  "Login failure",
		Category: model.TicketCategoryGeneral,
		Priority: model.TicketPriorityNormal,
		Body:     "The sign-in page returns an error.",
	})
	require.NoError(t, err)
	require.NotNil(t, ticket)
	assert.Equal(t, 7, ticket.UserID)
	assert.Equal(t, model.TicketStatusOpen, ticket.Status)
	assert.Equal(t, 1, ticket.MessageCount)
	assert.Equal(t, 1, ticket.UnreadForAdmin)

	var messages []model.TicketMessage
	require.NoError(t, model.DB.Where("ticket_id = ?", ticket.ID).Find(&messages).Error)
	require.Len(t, messages, 1)
	assert.Equal(t, model.TicketAuthorRoleUser, messages[0].AuthorRole)
}

func TestTicketConversationTransitionsAndClosedReplyConflict(t *testing.T) {
	setupTicketServiceTestDB(t)
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{
		Subject: "Refund question", Category: model.TicketCategoryRefund, Priority: model.TicketPriorityHigh, Body: "I need a manual refund review.",
	})
	require.NoError(t, err)

	adminMessage, err := AddTicketMessage(context.Background(), ticket.ID, 1, model.TicketAuthorRoleAdmin, "Please send the order number.")
	require.NoError(t, err)
	assert.Equal(t, model.TicketAuthorRoleAdmin, adminMessage.AuthorRole)
	detail, err := GetTicketDetail(context.Background(), ticket.ID, 7, false)
	require.NoError(t, err)
	assert.Equal(t, model.TicketStatusPendingUser, detail.Ticket.Status)
	assert.Equal(t, 2, detail.Ticket.MessageCount)

	_, err = AddTicketMessage(context.Background(), ticket.ID, 7, model.TicketAuthorRoleUser, "Order number is 123.")
	require.NoError(t, err)
	detail, err = GetTicketDetail(context.Background(), ticket.ID, 7, false)
	require.NoError(t, err)
	assert.Equal(t, model.TicketStatusPendingAdmin, detail.Ticket.Status)

	_, err = UpdateTicketState(context.Background(), ticket.ID, model.TicketAuthorRoleAdmin, stringPtr(model.TicketStatusClosed), nil)
	require.NoError(t, err)
	_, err = AddTicketMessage(context.Background(), ticket.ID, 7, model.TicketAuthorRoleUser, "Another message")
	assert.ErrorIs(t, err, ErrTicketClosed)

	updated, err := UpdateTicketState(context.Background(), ticket.ID, model.TicketAuthorRoleAdmin, stringPtr(model.TicketStatusPendingAdmin), nil)
	require.NoError(t, err)
	assert.Equal(t, model.TicketStatusPendingAdmin, updated.Status)
}

func TestTicketReadsEnforceOwnershipAndAllowAdmin(t *testing.T) {
	setupTicketServiceTestDB(t)
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{
		Subject: "Private", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "Only owner can read this.",
	})
	require.NoError(t, err)

	_, err = GetTicketDetail(context.Background(), ticket.ID, 8, false)
	assert.ErrorIs(t, err, ErrTicketNotFound)
	adminDetail, err := GetTicketDetail(context.Background(), ticket.ID, 8, true)
	require.NoError(t, err)
	assert.Equal(t, ticket.ID, adminDetail.Ticket.ID)

	items, total, err := ListTickets(context.Background(), 8, false, TicketListFilter{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Zero(t, total)
	items, total, err = ListTickets(context.Background(), 8, true, TicketListFilter{})
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func stringPtr(value string) *string { return &value }

package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreTicketAttachmentKeepsFilesPrivateAndChecksOwnership(t *testing.T) {
	setupTicketServiceTestDB(t)
	root := t.TempDir()
	t.Setenv("YUAPI_TICKET_UPLOAD_ROOT", root)
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{Subject: "Attachment", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "See file."})
	require.NoError(t, err)
	var message model.TicketMessage
	require.NoError(t, model.DB.Where("ticket_id = ?", ticket.ID).First(&message).Error)

	attachment, err := StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 8, false, TicketAttachmentInput{
		FileName: "../../secret.txt", MIMEType: "text/plain", Reader: bytes.NewBufferString("private"), Size: int64(len("private")),
	})
	assert.Error(t, err)
	assert.Nil(t, attachment)
	_, err = StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, TicketAttachmentInput{
		FileName: "note.txt", MIMEType: "text/plain", Reader: bytes.NewBufferString("private"), Size: int64(len("private")),
	})
	require.NoError(t, err)

	stored, err := os.ReadDir(filepath.Join(root, "ticket", ""))
	assert.NoError(t, err)
	assert.NotEmpty(t, stored)
}

func TestStoreTicketAttachmentEnforcesCountAndMIME(t *testing.T) {
	setupTicketServiceTestDB(t)
	t.Setenv("YUAPI_TICKET_UPLOAD_ROOT", t.TempDir())
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{Subject: "Files", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "Files."})
	require.NoError(t, err)
	var message model.TicketMessage
	require.NoError(t, model.DB.Where("ticket_id = ?", ticket.ID).First(&message).Error)

	_, err = StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, TicketAttachmentInput{
		FileName: "script.exe", MIMEType: "application/x-msdownload", Reader: bytes.NewReader([]byte("bad")), Size: 3,
	})
	require.Error(t, err)
	for index := 0; index < 5; index++ {
		_, err = StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, TicketAttachmentInput{
			FileName: "note-" + string(rune('0'+index)) + ".txt", MIMEType: "text/plain", Reader: bytes.NewReader([]byte("ok")), Size: 2,
		})
		require.NoError(t, err)
	}
	_, err = StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, TicketAttachmentInput{
		FileName: "sixth.txt", MIMEType: "text/plain", Reader: bytes.NewReader([]byte("ok")), Size: 2,
	})
	require.Error(t, err)
}

func TestStoreTicketAttachmentRejectsHeaderInjectionName(t *testing.T) {
	setupTicketServiceTestDB(t)
	t.Setenv("YUAPI_TICKET_UPLOAD_ROOT", t.TempDir())
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{Subject: "Name", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "Name."})
	require.NoError(t, err)
	var message model.TicketMessage
	require.NoError(t, model.DB.Where("ticket_id = ?", ticket.ID).First(&message).Error)
	_, err = StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, TicketAttachmentInput{
		FileName: "bad\"\r\nX.txt", MIMEType: "text/plain", Reader: bytes.NewReader([]byte("bad")), Size: 3,
	})
	assert.Error(t, err)
}

func TestOpenTicketAttachmentAllowsOwnerAndAdminOnly(t *testing.T) {
	setupTicketServiceTestDB(t)
	t.Setenv("YUAPI_TICKET_UPLOAD_ROOT", t.TempDir())
	ticket, err := CreateTicket(context.Background(), 7, CreateTicketInput{Subject: "Download", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "Download."})
	require.NoError(t, err)
	var message model.TicketMessage
	require.NoError(t, model.DB.Where("ticket_id = ?", ticket.ID).First(&message).Error)
	attachment, err := StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, TicketAttachmentInput{
		FileName: "note.txt", MIMEType: "text/plain", Reader: bytes.NewReader([]byte("private")), Size: 7,
	})
	require.NoError(t, err)

	_, _, err = OpenTicketAttachment(context.Background(), ticket.ID, attachment.ID, 8, false)
	assert.ErrorIs(t, err, ErrTicketNotFound)
	reader, metadata, err := OpenTicketAttachment(context.Background(), ticket.ID, attachment.ID, 7, false)
	require.NoError(t, err)
	require.NotNil(t, reader)
	require.Equal(t, "note.txt", metadata.DisplayName)
	require.NoError(t, reader.Close())
	reader, _, err = OpenTicketAttachment(context.Background(), ticket.ID, attachment.ID, 8, true)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
}

package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	ticketservice "github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTicketControllerTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:ticket_controller_"+strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Ticket{}, &model.TicketMessage{}, &model.TicketAttachment{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
}

func ticketControllerContext(method, path, body string, userID, role int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userID)
	c.Set("role", role)
	return c, recorder
}

func TestCreateTicketHandlerCreatesManualRefundFeedback(t *testing.T) {
	setupTicketControllerTestDB(t)
	c, recorder := ticketControllerContext(http.MethodPost, "/api/tickets", `{"subject":"Manual refund","category":"refund","priority":"high","body":"Please review this order."}`, 7, common.RoleCommonUser)

	CreateTicket(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"category":"refund"`)
	assert.Contains(t, recorder.Body.String(), `"message_id"`)
	assert.NotContains(t, recorder.Body.String(), "balance")
}

func TestGetTicketHandlerHidesForeignTicket(t *testing.T) {
	setupTicketControllerTestDB(t)
	ticket, err := ticketservice.CreateTicket(context.Background(), 7, ticketservice.CreateTicketInput{Subject: "Private", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "Private body"})
	require.NoError(t, err)
	c, recorder := ticketControllerContext(http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), "", 8, common.RoleCommonUser)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(ticket.ID, 10)}}

	GetTicket(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminReplyAndCloseTicketHandler(t *testing.T) {
	setupTicketControllerTestDB(t)
	ticket, err := ticketservice.CreateTicket(context.Background(), 7, ticketservice.CreateTicketInput{Subject: "Review", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "Need help"})
	require.NoError(t, err)

	c, recorder := ticketControllerContext(http.MethodPost, "/api/admin/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/messages", `{"body":"We are checking it."}`, 1, common.RoleAdminUser)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(ticket.ID, 10)}}
	AdminReplyTicket(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	c, recorder = ticketControllerContext(http.MethodPatch, "/api/admin/tickets/"+strconv.FormatInt(ticket.ID, 10), `{"status":"closed"}`, 1, common.RoleAdminUser)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(ticket.ID, 10)}}
	AdminUpdateTicket(c)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetTicketHandlerIncludesAttachmentsOnTheirMessage(t *testing.T) {
	setupTicketControllerTestDB(t)
	t.Setenv("YUAPI_TICKET_UPLOAD_ROOT", t.TempDir())
	ticket, err := ticketservice.CreateTicket(context.Background(), 7, ticketservice.CreateTicketInput{Subject: "Attachment", Category: model.TicketCategoryGeneral, Priority: model.TicketPriorityNormal, Body: "See file"})
	require.NoError(t, err)
	var message model.TicketMessage
	require.NoError(t, model.DB.Where("ticket_id = ?", ticket.ID).First(&message).Error)
	_, err = ticketservice.StoreTicketAttachment(context.Background(), ticket.ID, message.ID, 7, false, ticketservice.TicketAttachmentInput{FileName: "note.txt", MIMEType: "text/plain", Reader: bytes.NewBufferString("hello"), Size: 5})
	require.NoError(t, err)
	c, recorder := ticketControllerContext(http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), "", 7, common.RoleCommonUser)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(ticket.ID, 10)}}
	GetTicket(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Data ticketDetailResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Messages, 1)
	assert.Len(t, payload.Data.Messages[0].Attachments, 1)
}

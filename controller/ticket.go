package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	ticketservice "github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type ticketCreateRequest struct {
	Subject  string `json:"subject"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Body     string `json:"body"`
}

type ticketReplyRequest struct {
	Body string `json:"body"`
}

type ticketUpdateRequest struct {
	Status   *string `json:"status,omitempty"`
	Priority *string `json:"priority,omitempty"`
}

type ticketSummaryResponse struct {
	ID             int64  `json:"id"`
	UserID         int    `json:"user_id,omitempty"`
	Subject        string `json:"subject"`
	Category       string `json:"category"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	MessageCount   int    `json:"message_count"`
	UnreadForUser  int    `json:"unread_for_user,omitempty"`
	UnreadForAdmin int    `json:"unread_for_admin,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	LastMessageAt  int64  `json:"last_message_at"`
}

type ticketAttachmentResponse struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	MIMEType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"created_at"`
}

type ticketMessageResponse struct {
	ID          int64                      `json:"id"`
	AuthorRole  string                     `json:"author_role"`
	Body        string                     `json:"body"`
	CreatedAt   int64                      `json:"created_at"`
	Attachments []ticketAttachmentResponse `json:"attachments,omitempty"`
}

type ticketDetailResponse struct {
	Ticket      ticketSummaryResponse      `json:"ticket"`
	Messages    []ticketMessageResponse    `json:"messages"`
	Attachments []ticketAttachmentResponse `json:"attachments,omitempty"`
}

func ListTickets(c *gin.Context) {
	items, total, err := ticketservice.ListTickets(c.Request.Context(), c.GetInt("id"), false, ticketListFilterFromQuery(c))
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": ticketSummaryResponses(items), "total": total, "page": ticketPage(c), "page_size": ticketPageSize(c)})
}

func CreateTicket(c *gin.Context) {
	var request ticketCreateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		ticketAPIError(c, errors.New("invalid ticket request"))
		return
	}
	ticket, err := ticketservice.CreateTicket(c.Request.Context(), c.GetInt("id"), ticketservice.CreateTicketInput{
		Subject: request.Subject, Category: request.Category, Priority: request.Priority, Body: request.Body,
	})
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, ticketSummaryResponseFromModel(*ticket))
}

func GetTicket(c *gin.Context) {
	GetTicketForRole(c, false)
}

func GetTicketForRole(c *gin.Context, isAdmin bool) {
	ticketID, err := ticketIDFromParam(c)
	if err != nil {
		ticketAPIError(c, ErrInvalidTicketID)
		return
	}
	detail, err := ticketservice.GetTicketDetail(c.Request.Context(), ticketID, c.GetInt("id"), isAdmin)
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, ticketDetailResponseFromService(*detail))
}

func ReplyTicket(c *gin.Context) {
	ReplyTicketForRole(c, model.TicketAuthorRoleUser)
}

func ReplyTicketForRole(c *gin.Context, authorRole string) {
	ticketID, err := ticketIDFromParam(c)
	if err != nil {
		ticketAPIError(c, ErrInvalidTicketID)
		return
	}
	var request ticketReplyRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		ticketAPIError(c, errors.New("invalid ticket reply"))
		return
	}
	message, err := ticketservice.AddTicketMessage(c.Request.Context(), ticketID, c.GetInt("id"), authorRole, request.Body)
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, ticketMessageResponseFromModel(*message))
}

func AdminListTickets(c *gin.Context) {
	items, total, err := ticketservice.ListTickets(c.Request.Context(), c.GetInt("id"), true, ticketListFilterFromQuery(c))
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": ticketSummaryResponses(items), "total": total, "page": ticketPage(c), "page_size": ticketPageSize(c)})
}

func AdminGetTicket(c *gin.Context) {
	GetTicketForRole(c, true)
}

func AdminReplyTicket(c *gin.Context) {
	ReplyTicketForRole(c, model.TicketAuthorRoleAdmin)
}

func AdminUpdateTicket(c *gin.Context) {
	ticketID, err := ticketIDFromParam(c)
	if err != nil {
		ticketAPIError(c, ErrInvalidTicketID)
		return
	}
	var request ticketUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		ticketAPIError(c, errors.New("invalid ticket update"))
		return
	}
	ticket, err := ticketservice.UpdateTicketState(c.Request.Context(), ticketID, model.TicketAuthorRoleAdmin, request.Status, request.Priority)
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, ticketSummaryResponseFromModel(*ticket))
}

func UploadTicketAttachment(c *gin.Context) {
	ticketID, err := ticketIDFromParam(c)
	if err != nil {
		ticketAPIError(c, ErrInvalidTicketID)
		return
	}
	messageID, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("message_id")), 10, 64)
	if err != nil || messageID <= 0 {
		ticketAPIError(c, errors.New("message_id is required"))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		ticketAPIError(c, errors.New("ticket attachment is required"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		ticketAPIError(c, errors.New("ticket attachment cannot be opened"))
		return
	}
	defer file.Close()
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	attachment, err := ticketservice.StoreTicketAttachment(c.Request.Context(), ticketID, messageID, c.GetInt("id"), isAdmin, ticketservice.TicketAttachmentInput{
		FileName: fileHeader.Filename, MIMEType: fileHeader.Header.Get("Content-Type"), Reader: file, Size: fileHeader.Size,
	})
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	common.ApiSuccess(c, ticketAttachmentResponseFromModel(*attachment))
}

func DownloadTicketAttachment(c *gin.Context) {
	ticketID, err := ticketIDFromParam(c)
	if err != nil {
		ticketAPIError(c, ErrInvalidTicketID)
		return
	}
	attachmentID, err := strconv.ParseInt(strings.TrimSpace(c.Param("attachment_id")), 10, 64)
	if err != nil || attachmentID <= 0 {
		ticketAPIError(c, ErrInvalidTicketID)
		return
	}
	file, attachment, err := ticketservice.OpenTicketAttachment(c.Request.Context(), ticketID, attachmentID, c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser)
	if err != nil {
		ticketAPIError(c, err)
		return
	}
	defer file.Close()
	c.DataFromReader(http.StatusOK, attachment.Size, attachment.MIMEType, file, map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, attachment.DisplayName),
	})
}

var ErrInvalidTicketID = errors.New("invalid ticket id")

func ticketIDFromParam(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidTicketID
	}
	return id, nil
}

func ticketListFilterFromQuery(c *gin.Context) ticketservice.TicketListFilter {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return ticketservice.TicketListFilter{Status: c.Query("status"), Category: c.Query("category"), Priority: c.Query("priority"), Keyword: c.Query("keyword"), Page: page, PageSize: pageSize}
}

func ticketPage(c *gin.Context) int {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		return 1
	}
	return page
}

func ticketPageSize(c *gin.Context) int {
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 100 {
		return 20
	}
	return pageSize
}

func ticketSummaryResponses(items []model.Ticket) []ticketSummaryResponse {
	responses := make([]ticketSummaryResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, ticketSummaryResponseFromModel(item))
	}
	return responses
}

func ticketSummaryResponseFromModel(ticket model.Ticket) ticketSummaryResponse {
	return ticketSummaryResponse{ID: ticket.ID, UserID: ticket.UserID, Subject: ticket.Subject, Category: ticket.Category, Status: ticket.Status, Priority: ticket.Priority, MessageCount: ticket.MessageCount, UnreadForUser: ticket.UnreadForUser, UnreadForAdmin: ticket.UnreadForAdmin, CreatedAt: ticket.CreatedAt, UpdatedAt: ticket.UpdatedAt, LastMessageAt: ticket.LastMessageAt}
}

func ticketDetailResponseFromService(detail ticketservice.TicketDetail) ticketDetailResponse {
	response := ticketDetailResponse{Ticket: ticketSummaryResponseFromModel(detail.Ticket), Messages: make([]ticketMessageResponse, 0, len(detail.Messages)), Attachments: make([]ticketAttachmentResponse, 0, len(detail.Attachments))}
	for _, message := range detail.Messages {
		response.Messages = append(response.Messages, ticketMessageResponseFromModel(message))
	}
	for _, attachment := range detail.Attachments {
		response.Attachments = append(response.Attachments, ticketAttachmentResponseFromModel(attachment))
	}
	return response
}

func ticketMessageResponseFromModel(message model.TicketMessage) ticketMessageResponse {
	return ticketMessageResponse{ID: message.ID, AuthorRole: message.AuthorRole, Body: message.Body, CreatedAt: message.CreatedAt}
}

func ticketAttachmentResponseFromModel(attachment model.TicketAttachment) ticketAttachmentResponse {
	return ticketAttachmentResponse{ID: attachment.ID, DisplayName: attachment.DisplayName, MIMEType: attachment.MIMEType, Size: attachment.Size, CreatedAt: attachment.CreatedAt}
}

func ticketAPIError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	message := err.Error()
	switch {
	case errors.Is(err, ticketservice.ErrTicketNotFound), errors.Is(err, ErrInvalidTicketID):
		status = http.StatusNotFound
		message = "ticket not found"
	case errors.Is(err, ticketservice.ErrTicketClosed):
		status = http.StatusConflict
		message = "ticket is closed"
	default:
		if strings.Contains(strings.ToLower(message), "database") || strings.Contains(strings.ToLower(message), "sql") {
			status = http.StatusInternalServerError
			message = "ticket service unavailable"
		}
	}
	c.JSON(status, gin.H{"success": false, "message": message})
}

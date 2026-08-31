package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	maxTicketAttachmentsPerMessage = 5
	maxTicketAttachmentBytes       = int64(50 << 20)
)

type TicketAttachmentInput struct {
	FileName string
	MIMEType string
	Reader   io.Reader
	Size     int64
}

func StoreTicketAttachment(ctx context.Context, ticketID, messageID int64, uploaderID int, isAdmin bool, input TicketAttachmentInput) (*model.TicketAttachment, error) {
	if ticketID <= 0 || messageID <= 0 || uploaderID <= 0 {
		return nil, ErrTicketNotFound
	}
	fileName, err := safeTicketAttachmentName(input.FileName)
	if err != nil {
		return nil, err
	}
	if input.Reader == nil || input.Size <= 0 || input.Size > maxTicketAttachmentBytes {
		return nil, errors.New("invalid ticket attachment size")
	}
	mimeType := normalizeTicketMIMEType(input.MIMEType)
	if !allowedTicketMIMEType(mimeType) {
		return nil, errors.New("unsupported ticket attachment type")
	}

	var attachment *model.TicketAttachment
	err = model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ticket model.Ticket
		query := tx.Where("id = ?", ticketID)
		if !isAdmin {
			query = query.Where("user_id = ?", uploaderID)
		}
		if err := query.First(&ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTicketNotFound
			}
			return err
		}
		var message model.TicketMessage
		if err := tx.Where("id = ? AND ticket_id = ?", messageID, ticketID).First(&message).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTicketNotFound
			}
			return err
		}
		var count int64
		if err := tx.Model(&model.TicketAttachment{}).Where("message_id = ?", messageID).Count(&count).Error; err != nil {
			return err
		}
		if count >= maxTicketAttachmentsPerMessage {
			return errors.New("ticket message attachment limit reached")
		}

		storageKey, written, storeErr := storeTicketAttachmentFile(input.Reader, ticketID, fileName, mimeType)
		if storeErr != nil {
			return storeErr
		}
		attachment = &model.TicketAttachment{
			TicketID: ticketID, MessageID: messageID, UploaderID: uploaderID,
			StorageKey: storageKey, DisplayName: fileName, MIMEType: mimeType, Size: written, CreatedAt: common.GetTimestamp(),
		}
		if err := tx.Create(attachment).Error; err != nil {
			_ = os.Remove(ticketAttachmentAbsolutePath(storageKey))
			attachment = nil
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return attachment, nil
}

func OpenTicketAttachment(ctx context.Context, ticketID, attachmentID int64, userID int, isAdmin bool) (io.ReadCloser, *model.TicketAttachment, error) {
	if ticketID <= 0 || attachmentID <= 0 || userID <= 0 {
		return nil, nil, ErrTicketNotFound
	}
	var attachment model.TicketAttachment
	query := model.DB.WithContext(ctx).Where("id = ? AND ticket_id = ?", attachmentID, ticketID)
	if !isAdmin {
		query = query.Where("uploader_id = ? OR ticket_id IN (SELECT id FROM tickets WHERE user_id = ?)", userID, userID)
	}
	if err := query.First(&attachment).Error; err != nil {
		return nil, nil, ErrTicketNotFound
	}
	pathValue, err := safeTicketAttachmentPath(attachment.StorageKey)
	if err != nil {
		return nil, nil, ErrTicketNotFound
	}
	file, err := os.Open(pathValue)
	if err != nil {
		return nil, nil, ErrTicketNotFound
	}
	return file, &attachment, nil
}

func storeTicketAttachmentFile(reader io.Reader, ticketID int64, fileName, mimeType string) (string, int64, error) {
	root := ticketAttachmentRoot()
	directory := filepath.Join(root, "ticket", fmt.Sprintf("%d", ticketID))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", 0, err
	}
	_ = os.Chmod(directory, 0o700)
	temporary, err := os.CreateTemp(directory, ".ticket-attachment-*.part")
	if err != nil {
		return "", 0, err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return "", 0, err
	}
	written, err := io.Copy(temporary, io.LimitReader(reader, maxTicketAttachmentBytes+1))
	if err != nil {
		return "", written, err
	}
	if written <= 0 || written > maxTicketAttachmentBytes {
		return "", written, errors.New("invalid ticket attachment size")
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		return "", written, err
	}
	prefix := make([]byte, 512)
	prefixLength, err := temporary.Read(prefix)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", written, err
	}
	if !allowedTicketContentType(http.DetectContentType(prefix[:prefixLength]), mimeType) {
		return "", written, errors.New("ticket attachment content type mismatch")
	}
	randomKey, err := common.GenerateRandomCharsKey(24)
	if err != nil || strings.TrimSpace(randomKey) == "" {
		return "", written, errors.New("failed to generate ticket attachment key")
	}
	storageKey := filepath.ToSlash(filepath.Join("ticket", fmt.Sprintf("%d", ticketID), randomKey))
	finalPath, err := safeTicketAttachmentPath(storageKey)
	if err != nil {
		return "", written, err
	}
	if err := temporary.Close(); err != nil {
		return "", written, err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", written, err
	}
	return storageKey, written, nil
}

func ticketAttachmentRoot() string {
	if configured := strings.TrimSpace(common.GetEnvOrDefaultString("YUAPI_TICKET_UPLOAD_ROOT", "")); configured != "" {
		return configured
	}
	return filepath.Join(os.TempDir(), "yuapi-ticket-uploads")
}

func safeTicketAttachmentName(value string) (string, error) {
	value = strings.TrimSpace(value)
	clean := filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || clean == "." || clean == "/" || clean != value || strings.ContainsAny(clean, "\"\r\n") {
		return "", errors.New("invalid ticket attachment name")
	}
	if len([]rune(clean)) > 255 {
		return "", errors.New("ticket attachment name is too long")
	}
	return clean, nil
}

func safeTicketAttachmentPath(storageKey string) (string, error) {
	root, err := filepath.Abs(ticketAttachmentRoot())
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.FromSlash(storageKey))
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if absolute != root && !strings.HasPrefix(absolute, root+string(os.PathSeparator)) {
		return "", errors.New("invalid ticket attachment path")
	}
	return absolute, nil
}

func ticketAttachmentAbsolutePath(storageKey string) string {
	pathValue, err := safeTicketAttachmentPath(storageKey)
	if err != nil {
		return ""
	}
	return pathValue
}

func normalizeTicketMIMEType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}

func allowedTicketMIMEType(value string) bool {
	switch value {
	case "text/plain", "text/markdown", "application/pdf", "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func allowedTicketContentType(detected, declared string) bool {
	if detected == "application/octet-stream" {
		return false
	}
	if detected == "text/plain; charset=utf-8" && (declared == "text/plain" || declared == "text/markdown") {
		return true
	}
	return strings.HasPrefix(detected, "image/") && strings.HasPrefix(declared, "image/") || detected == declared
}

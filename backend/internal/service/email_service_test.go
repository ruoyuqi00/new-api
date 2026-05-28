package service

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailServicePlainSMTPWithoutAuth(t *testing.T) {
	smtpServer := startNotificationEmailTestSMTPServer(t)
	host, portText, err := net.SplitHostPort(smtpServer.listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	svc := NewEmailService(newNotificationEmailMemorySettingRepo(), nil)
	err = svc.SendEmailWithConfig(&SMTPConfig{
		Host:     host,
		Port:     port,
		From:     "noreply@example.com",
		FromName: "Sub2API",
		UseTLS:   false,
	}, "user@example.com", "Test", "<p>ok</p>")
	require.NoError(t, err)
	require.Equal(t, int64(1), smtpServer.messageCount())
}

func TestEmailServicePlainSMTPConnectionTestWithoutAuth(t *testing.T) {
	smtpServer := startNotificationEmailTestSMTPServer(t)
	host, portText, err := net.SplitHostPort(smtpServer.listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	svc := NewEmailService(newNotificationEmailMemorySettingRepo(), nil)
	err = svc.TestSMTPConnectionWithConfig(&SMTPConfig{
		Host:   host,
		Port:   port,
		UseTLS: false,
	})
	require.NoError(t, err)
}

func TestEmailServiceSMTPAuthNilWhenCredentialsEmpty(t *testing.T) {
	require.Nil(t, smtpAuthForConfig(&SMTPConfig{}))
	require.Nil(t, smtpAuthForConfig(&SMTPConfig{Username: " ", Password: " "}))
	require.NotNil(t, smtpAuthForConfig(&SMTPConfig{Host: "smtp.example.com", Username: "user"}))
	require.NotNil(t, smtpAuthForConfig(&SMTPConfig{Host: "smtp.example.com", Password: "secret"}))
}

func TestEmailServiceSendEmailUsesSavedRelayConfigWithoutAuth(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	host, port, _ := net.SplitHostPort(smtpServer.listener.Addr().String())
	require.NoError(t, repo.SetMultiple(ctx, map[string]string{
		SettingKeySMTPHost:     host,
		SettingKeySMTPPort:     port,
		SettingKeySMTPFrom:     "noreply@example.com",
		SettingKeySMTPFromName: "Sub2API",
		SettingKeySMTPUseTLS:   "false",
	}))

	svc := NewEmailService(repo, nil)
	require.NoError(t, svc.SendEmail(ctx, "user@example.com", "Test", "<p>ok</p>"))
	require.Equal(t, int64(1), smtpServer.messageCount())
}

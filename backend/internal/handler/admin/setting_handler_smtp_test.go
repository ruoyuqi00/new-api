package admin

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_TestSMTPConnection_AllowsExplicitEmptyCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	smtpServer := startSettingHandlerNoAuthSMTPServer(t)
	host, portText, err := net.SplitHostPort(smtpServer.listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeySMTPHost:     host,
		service.SettingKeySMTPPort:     portText,
		service.SettingKeySMTPUsername: "old-user",
		service.SettingKeySMTPPassword: "old-pass",
		service.SettingKeySMTPFrom:     "old@example.com",
		service.SettingKeySMTPFromName: "Old",
		service.SettingKeySMTPUseTLS:   "false",
	}}
	settingSvc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	emailSvc := service.NewEmailService(repo, nil)
	handler := NewSettingHandler(settingSvc, emailSvc, nil, nil, nil, nil, nil)

	router := gin.New()
	router.POST("/test-smtp", handler.TestSMTPConnection)
	req := httptest.NewRequest(http.MethodPost, "/test-smtp", stringsReader(`{
		"smtp_host":"`+host+`",
		"smtp_port":`+strconv.Itoa(port)+`,
		"smtp_username":"",
		"smtp_password":"",
		"smtp_use_tls":false
	}`))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, int64(0), smtpServer.authCount.Load())
}

func TestSettingHandler_SendTestEmail_AllowsExplicitEmptyCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	smtpServer := startSettingHandlerNoAuthSMTPServer(t)
	host, portText, err := net.SplitHostPort(smtpServer.listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeySiteName:     "Sub2API",
		service.SettingKeySMTPHost:     host,
		service.SettingKeySMTPPort:     portText,
		service.SettingKeySMTPUsername: "old-user",
		service.SettingKeySMTPPassword: "old-pass",
		service.SettingKeySMTPFrom:     "old@example.com",
		service.SettingKeySMTPFromName: "Old",
		service.SettingKeySMTPUseTLS:   "false",
	}}
	settingSvc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	emailSvc := service.NewEmailService(repo, nil)
	handler := NewSettingHandler(settingSvc, emailSvc, nil, nil, nil, nil, nil)

	router := gin.New()
	router.POST("/send-test-email", handler.SendTestEmail)
	req := httptest.NewRequest(http.MethodPost, "/send-test-email", stringsReader(`{
		"email":"user@example.com",
		"smtp_host":"`+host+`",
		"smtp_port":`+strconv.Itoa(port)+`,
		"smtp_username":"",
		"smtp_password":"",
		"smtp_from_email":"no-reply@example.com",
		"smtp_from_name":"Sub2API",
		"smtp_use_tls":false
	}`))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, int64(0), smtpServer.authCount.Load())
	require.Equal(t, int64(1), smtpServer.messageCount.Load())
}

func stringsReader(s string) *strings.Reader {
	return strings.NewReader(s)
}

type settingHandlerNoAuthSMTPServer struct {
	listener     net.Listener
	authCount    atomic.Int64
	messageCount atomic.Int64
}

func startSettingHandlerNoAuthSMTPServer(t *testing.T) *settingHandlerNoAuthSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &settingHandlerNoAuthSMTPServer{listener: listener}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.handle(conn)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	return server
}

func (s *settingHandlerNoAuthSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	writeLine := func(line string) bool {
		if _, err := rw.WriteString(line + "\r\n"); err != nil {
			return false
		}
		return rw.Flush() == nil
	}
	if !writeLine("220 localhost ESMTP") {
		return
	}
	for {
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimRight(line, "\r\n"))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			if !writeLine("250 localhost") {
				return
			}
		case strings.HasPrefix(cmd, "AUTH"):
			s.authCount.Add(1)
			_ = writeLine("538 5.7.11 Encryption required for requested authentication mechanism")
			return
		case strings.HasPrefix(cmd, "MAIL FROM:"), strings.HasPrefix(cmd, "RCPT TO:"):
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(cmd, "DATA"):
			if !writeLine("354 End data with <CR><LF>.<CR><LF>") {
				return
			}
			for {
				dataLine, err := rw.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
			}
			s.messageCount.Add(1)
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(cmd, "QUIT"):
			_ = writeLine("221 Bye")
			return
		default:
			if !writeLine("250 OK") {
				return
			}
		}
	}
}

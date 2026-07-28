package service

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
)

const (
	http2ReadIdleTimeout = 15 * time.Second
	http2PingTimeout     = 15 * time.Second
)

var (
	httpClient   *http.Client
	proxyClients = proxyHTTPClientCache{
		clients: make(map[string]*http.Client),
		aliases: make(map[string]string),
	}
)

type proxyHTTPClientCache struct {
	mutex   sync.RWMutex
	clients map[string]*http.Client
	aliases map[string]string
}

type proxyURLConfig struct {
	parsedURL *url.URL
	cacheKey  string
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := system_setting.GetFetchSetting()
	urlStr := req.URL.String()
	if err := common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func newRelayHTTPTransport() (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:     time.Duration(common.RelayIdleConnTimeout) * time.Second,
		ForceAttemptHTTP2:   true,
		Proxy:               http.ProxyFromEnvironment,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}
	if _, err := enableHTTP2KeepAlive(transport); err != nil {
		return transport, fmt.Errorf("enable HTTP/2 keep-alive: %w", err)
	}
	return transport, nil
}

func newRelayHTTPClient(transport *http.Transport) *http.Client {
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
	if common.RelayTimeout != 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

func InitHttpClient() {
	transport, err := newRelayHTTPTransport()
	if err != nil {
		common.SysError(err.Error())
	}
	httpClient = newRelayHTTPClient(transport)
}

func GetHttpClient() *http.Client {
	return httpClient
}

func newProxyURLConfig(parsedURL *url.URL) *proxyURLConfig {
	return &proxyURLConfig{parsedURL: parsedURL, cacheKey: parsedURL.String()}
}

// NormalizeProxyURL returns the canonical cache key used for a proxy URL.
func NormalizeProxyURL(rawProxyURL string) (string, error) {
	parsedURL, _, err := common.ParseProxyURLRuntime(rawProxyURL)
	if err != nil {
		return "", err
	}
	if parsedURL == nil {
		return "", nil
	}
	return newProxyURLConfig(parsedURL).cacheKey, nil
}

// ValidateProxyURL checks a proxy URL without connecting to it.
func ValidateProxyURL(rawProxyURL string) error {
	_, err := common.ParseProxyURLStrict(rawProxyURL)
	return err
}

func (cache *proxyHTTPClientCache) get(rawCacheKey string) (*http.Client, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	cacheKey := rawCacheKey
	if canonicalKey, ok := cache.aliases[rawCacheKey]; ok {
		cacheKey = canonicalKey
	}
	client, ok := cache.clients[cacheKey]
	return client, ok
}

func (cache *proxyHTTPClientCache) getOrCreate(rawCacheKey string, config *proxyURLConfig) (*http.Client, error) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if client, ok := cache.clients[config.cacheKey]; ok {
		cache.aliases[rawCacheKey] = config.cacheKey
		return client, nil
	}

	client, err := newProxyHTTPClient(config.parsedURL)
	if err != nil {
		return nil, err
	}
	cache.clients[config.cacheKey] = client
	cache.aliases[rawCacheKey] = config.cacheKey
	return client, nil
}

func (cache *proxyHTTPClientCache) remove(cacheKey string) *http.Client {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	client := cache.clients[cacheKey]
	delete(cache.clients, cacheKey)
	for alias, canonicalKey := range cache.aliases {
		if canonicalKey == cacheKey {
			delete(cache.aliases, alias)
		}
	}
	return client
}

func (cache *proxyHTTPClientCache) reset() map[string]*http.Client {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	oldClients := cache.clients
	cache.clients = make(map[string]*http.Client)
	cache.aliases = make(map[string]string)
	return oldClients
}

func newProxyHTTPClient(proxyURL *url.URL) (*http.Client, error) {
	transport, err := newRelayHTTPTransport()
	if err != nil {
		return nil, err
	}

	switch proxyURL.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(proxyURL)
	case "socks5", "socks5h":
		transport.Proxy = nil
		forwardDialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		dialer, err := proxy.FromURL(proxyURL, forwardDialer)
		if err != nil {
			return nil, err
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS proxy dialer does not support context cancellation")
		}
		transport.DialContext = contextDialer.DialContext
	default:
		return nil, fmt.Errorf("unsupported proxy scheme")
	}

	return newRelayHTTPClient(transport), nil
}

// GetHttpClientWithProxy returns the default client or a cached proxy-enabled client.
func GetHttpClientWithProxy(rawProxyURL string) (*http.Client, error) {
	trimmedProxyURL := strings.TrimSpace(rawProxyURL)
	if trimmedProxyURL == "" {
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}
	if client, ok := proxyClients.get(trimmedProxyURL); ok {
		return client, nil
	}

	parsedURL, _, err := common.ParseProxyURLRuntime(trimmedProxyURL)
	if err != nil {
		return nil, err
	}
	return proxyClients.getOrCreate(trimmedProxyURL, newProxyURLConfig(parsedURL))
}

// InvalidateProxyClient removes one proxy client and closes its idle connections.
func InvalidateProxyClient(rawProxyURL string) {
	parsedURL, _, err := common.ParseProxyURLRuntime(rawProxyURL)
	if err != nil || parsedURL == nil {
		return
	}
	if client := proxyClients.remove(newProxyURLConfig(parsedURL).cacheKey); client != nil {
		client.CloseIdleConnections()
	}
}

// ResetProxyClientCache clears all cached proxy clients.
func ResetProxyClientCache() {
	for _, client := range proxyClients.reset() {
		client.CloseIdleConnections()
	}
}

// NewProxyHttpClient is retained for compatibility with integrations outside this package.
// Deprecated: use GetHttpClientWithProxy.
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	return GetHttpClientWithProxy(proxyURL)
}

// enableHTTP2KeepAlive configures active PING health checks so pooled HTTP/2
// connections silently dropped by a proxy or NAT are evicted before reuse.
func enableHTTP2KeepAlive(transport *http.Transport) (*http2.Transport, error) {
	h2Transport, err := http2.ConfigureTransports(transport)
	if err != nil {
		return nil, err
	}
	h2Transport.ReadIdleTimeout = http2ReadIdleTimeout
	h2Transport.PingTimeout = http2PingTimeout
	return h2Transport, nil
}

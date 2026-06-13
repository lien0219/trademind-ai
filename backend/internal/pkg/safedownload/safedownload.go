package safedownload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/image/webp"
)

const (
	ErrSchemeNotAllowed   = "SAFE_DOWNLOAD_SCHEME_NOT_ALLOWED"
	ErrCredentialsInURL   = "SAFE_DOWNLOAD_CREDENTIALS_IN_URL"
	ErrPrivateHost        = "SAFE_DOWNLOAD_PRIVATE_HOST"
	ErrPrivateIP          = "SAFE_DOWNLOAD_PRIVATE_IP"
	ErrMetadataEndpoint   = "SAFE_DOWNLOAD_METADATA_ENDPOINT"
	ErrTooManyRedirects   = "SAFE_DOWNLOAD_TOO_MANY_REDIRECTS"
	ErrResponseTooLarge   = "SAFE_DOWNLOAD_RESPONSE_TOO_LARGE"
	ErrInvalidContentType = "SAFE_DOWNLOAD_INVALID_CONTENT_TYPE"
	ErrImageDecodeFailed  = "SAFE_DOWNLOAD_IMAGE_DECODE_FAILED"
	ErrDownloadFailed     = "SAFE_DOWNLOAD_FAILED"
)

// Options configures safe HTTP download behavior.
type Options struct {
	MaxBodyBytes    int64
	MaxRedirects    int
	ConnectTimeout  time.Duration
	ResponseTimeout time.Duration
	RequireImage    bool
	UserAgent       string
}

// DefaultOptions returns conservative defaults for image download.
func DefaultOptions() Options {
	return Options{
		MaxBodyBytes:    10 << 20,
		MaxRedirects:    5,
		ConnectTimeout:  10 * time.Second,
		ResponseTimeout: 30 * time.Second,
		RequireImage:    true,
		UserAgent:       "TradeMind-SafeDownload/1.0",
	}
}

// Result holds downloaded bytes and metadata.
type Result struct {
	Data        []byte
	ContentType string
	FinalURL    string
}

// Download fetches rawURL with SSRF protections.
func Download(ctx context.Context, rawURL string, opts Options) (*Result, error) {
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 10 << 20
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = 5
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 10 * time.Second
	}
	if opts.ResponseTimeout <= 0 {
		opts.ResponseTimeout = 30 * time.Second
	}
	if strings.TrimSpace(opts.UserAgent) == "" {
		opts.UserAgent = "TradeMind-SafeDownload/1.0"
	}

	current := strings.TrimSpace(rawURL)
	redirects := 0
	for {
		if err := validateURL(ctx, current); err != nil {
			return nil, err
		}
		data, ct, loc, err := fetchOnce(ctx, current, opts)
		if err != nil {
			return nil, err
		}
		if loc != "" {
			redirects++
			if redirects > opts.MaxRedirects {
				return nil, fmt.Errorf("%s: exceeded %d redirects", ErrTooManyRedirects, opts.MaxRedirects)
			}
			current = loc
			continue
		}
		if opts.RequireImage {
			if err := validateImageBytes(data, ct); err != nil {
				return nil, err
			}
		}
		return &Result{Data: data, ContentType: ct, FinalURL: current}, nil
	}
}

func fetchOnce(ctx context.Context, rawURL string, opts Options) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: %w", ErrDownloadFailed, err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	cli := safeHTTPClient(opts)
	resp, err := cli.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: %w", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := strings.TrimSpace(resp.Header.Get("Location"))
		if loc == "" {
			return nil, "", "", fmt.Errorf("%s: redirect without location", ErrDownloadFailed)
		}
		abs, err := resolveRedirect(rawURL, loc)
		if err != nil {
			return nil, "", "", err
		}
		return nil, "", abs, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("%s: http %d", ErrDownloadFailed, resp.StatusCode)
	}

	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	data, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodyBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: %w", ErrDownloadFailed, err)
	}
	if int64(len(data)) > opts.MaxBodyBytes {
		return nil, "", "", fmt.Errorf("%s: body exceeds %d bytes", ErrResponseTooLarge, opts.MaxBodyBytes)
	}
	return data, ct, "", nil
}

func safeHTTPClient(opts Options) *http.Client {
	return &http.Client{
		Timeout: opts.ResponseTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					host = addr
				}
				if err := assertIPNotPrivate(net.ParseIP(host)); err != nil {
					return nil, err
				}
				d := &net.Dialer{Timeout: opts.ConnectTimeout, KeepAlive: 30 * time.Second}
				return d.DialContext(ctx, network, net.JoinHostPort(host, port))
			},
			TLSHandshakeTimeout: opts.ConnectTimeout,
			MaxIdleConns:        4,
		},
	}
}

func validateURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s: invalid url", ErrDownloadFailed)
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%s: only http/https allowed", ErrSchemeNotAllowed)
	}
	if u.User != nil {
		return fmt.Errorf("%s: credentials in url forbidden", ErrCredentialsInURL)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("%s: empty host", ErrDownloadFailed)
	}
	if isBlockedHost(host) {
		return fmt.Errorf("%s: host %s blocked", ErrPrivateHost, host)
	}
	if isMetadataHost(host) {
		return fmt.Errorf("%s: metadata endpoint blocked", ErrMetadataEndpoint)
	}
	return assertHostResolvedNotPrivate(ctx, host)
}

func isBlockedHost(host string) bool {
	hl := strings.ToLower(strings.TrimSpace(host))
	if hl == "localhost" || hl == "0.0.0.0" || strings.HasSuffix(hl, ".localhost") {
		return true
	}
	if ip := net.ParseIP(hl); ip != nil {
		return isPrivateIP(ip)
	}
	return false
}

func isMetadataHost(host string) bool {
	hl := strings.ToLower(strings.TrimSpace(host))
	return hl == "metadata.google.internal" ||
		hl == "169.254.169.254" ||
		strings.HasSuffix(hl, ".internal")
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Block CGNAT / shared address space and metadata
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 {
			return true
		}
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

func assertIPNotPrivate(ip net.IP) error {
	if isPrivateIP(ip) {
		return fmt.Errorf("%s: connection to private address blocked", ErrPrivateIP)
	}
	return nil
}

func assertHostResolvedNotPrivate(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		return assertIPNotPrivate(ip)
	}
	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%s: dns lookup failed: %w", ErrDownloadFailed, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%s: no dns records", ErrDownloadFailed)
	}
	for _, ia := range ips {
		if err := assertIPNotPrivate(ia.IP); err != nil {
			return err
		}
	}
	return nil
}

func resolveRedirect(base, loc string) (string, error) {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return "", fmt.Errorf("%s: empty redirect", ErrDownloadFailed)
	}
	if strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://") {
		return loc, nil
	}
	bu, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	lu, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	return bu.ResolveReference(lu).String(), nil
}

func validateImageBytes(data []byte, contentType string) error {
	if len(data) == 0 {
		return fmt.Errorf("%s: empty body", ErrImageDecodeFailed)
	}
	ct := strings.ToLower(strings.Split(strings.TrimSpace(contentType), ";")[0])
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("%s: content-type %s", ErrInvalidContentType, contentType)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if _, werr := webp.DecodeConfig(bytes.NewReader(data)); werr == nil {
			return nil
		}
		return fmt.Errorf("%s: %w", ErrImageDecodeFailed, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("%s: invalid dimensions", ErrImageDecodeFailed)
	}
	return nil
}

// IsPrivateIP reports whether ip is in a blocked range (exported for tests).
func IsPrivateIP(ip net.IP) bool {
	return isPrivateIP(ip)
}

// ValidateURL checks URL without downloading (exported for tests).
func ValidateURL(ctx context.Context, raw string) error {
	return validateURL(ctx, raw)
}

// ErrCode extracts a stable error code from download errors.
func ErrCode(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, code := range []string{
		ErrSchemeNotAllowed, ErrCredentialsInURL, ErrPrivateHost, ErrPrivateIP,
		ErrMetadataEndpoint, ErrTooManyRedirects, ErrResponseTooLarge,
		ErrInvalidContentType, ErrImageDecodeFailed, ErrDownloadFailed,
	} {
		if strings.Contains(msg, code) {
			return code
		}
	}
	return ErrDownloadFailed
}

// Wrap preserves code in error chain.
func Wrap(err error, code string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), code) {
		return err
	}
	return fmt.Errorf("%s: %w", code, err)
}

var errPrivate = errors.New("private")

func init() {
	_ = errPrivate
}

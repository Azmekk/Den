package service

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type UnfurlResult struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	Video       string `json:"video,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	Type        string `json:"type,omitempty"`
}

type cachedUnfurl struct {
	result    *UnfurlResult
	err       error
	fetchedAt time.Time
}

type UnfurlService struct {
	client    *http.Client
	cache     sync.Map
	userAgent string
}

const (
	unfurlCacheTTL   = 10 * time.Minute
	unfurlMaxBody    = 512 * 1024 // 512 KB - only need the <head>
	unfurlTimeout    = 5 * time.Second
)

func NewUnfurlService(userAgent string) *UnfurlService {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (compatible; DenBot/1.0)"
	}
	return &UnfurlService{
		userAgent: userAgent,
		client: &http.Client{
			Timeout: unfurlTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (s *UnfurlService) Unfurl(rawURL string) (*UnfurlResult, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("invalid URL scheme")
	}

	// Check cache
	if cached, ok := s.cache.Load(rawURL); ok {
		entry := cached.(*cachedUnfurl)
		if time.Since(entry.fetchedAt) < unfurlCacheTTL {
			return entry.result, entry.err
		}
		s.cache.Delete(rawURL)
	}

	result, err := s.fetch(rawURL)

	s.cache.Store(rawURL, &cachedUnfurl{
		result:    result,
		err:       err,
		fetchedAt: time.Now(),
	})

	return result, err
}

func (s *UnfurlService) fetch(rawURL string) (*UnfurlResult, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml") {
		return nil, fmt.Errorf("not HTML: %s", ct)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, unfurlMaxBody))
	if err != nil {
		return nil, err
	}

	html := string(body)
	result := &UnfurlResult{URL: rawURL}

	result.Title = extractMeta(html, "og:title")
	result.Description = extractMeta(html, "og:description")
	result.Image = extractMeta(html, "og:image")
	result.SiteName = extractMeta(html, "og:site_name")
	result.Type = extractMeta(html, "og:type")

	// Check for og:video, then og:video:url, then og:video:secure_url
	result.Video = extractMeta(html, "og:video:secure_url")
	if result.Video == "" {
		result.Video = extractMeta(html, "og:video:url")
	}
	if result.Video == "" {
		result.Video = extractMeta(html, "og:video")
	}

	// Fall back to <title> tag if no og:title
	if result.Title == "" {
		result.Title = extractTitle(html)
	}

	// If nothing useful was found, return nil
	if result.Title == "" && result.Description == "" && result.Image == "" && result.Video == "" {
		return nil, fmt.Errorf("no OG metadata found")
	}

	return result, nil
}

var metaRegex = regexp.MustCompile(`<meta\s[^>]*?>`)
var propertyRegex = regexp.MustCompile(`(?:property|name)\s*=\s*["']([^"']+)["']`)
var contentRegex = regexp.MustCompile(`content\s*=\s*["']([^"']*?)["']`)
var titleTagRegex = regexp.MustCompile(`<title[^>]*>(.*?)</title>`)

func extractMeta(html string, property string) string {
	matches := metaRegex.FindAllString(html, -1)
	for _, tag := range matches {
		propMatch := propertyRegex.FindStringSubmatch(tag)
		if propMatch == nil || propMatch[1] != property {
			continue
		}
		contentMatch := contentRegex.FindStringSubmatch(tag)
		if contentMatch != nil {
			return strings.TrimSpace(contentMatch[1])
		}
	}
	return ""
}

func extractTitle(html string) string {
	m := titleTagRegex.FindStringSubmatch(html)
	if m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

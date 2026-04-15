package bin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GitHubRateLimitError struct {
	URL            string
	StatusCode     int
	Status         string
	Remaining      string
	Message        string
	ResetAt        time.Time
	FirstURL       string
	ShortCircuited bool
}

func (e *GitHubRateLimitError) Error() string {
	if e == nil {
		return ""
	}

	action := "while fetching"
	if e.ShortCircuited {
		action = "before fetching"
	}

	parts := []string{fmt.Sprintf("%s %s %s", e.summaryPrefix(), action, e.URL)}
	if e.Status != "" {
		parts = append(parts, "HTTP "+e.Status)
	}
	if message := normalizeGitHubRateLimitMessage(e.Message); message != "" {
		parts = append(parts, message)
	}
	if detail := e.ResetSummary(); detail != "" {
		parts = append(parts, detail)
	}
	if e.ShortCircuited && e.FirstURL != "" && e.FirstURL != e.URL {
		parts = append(parts, "first seen at "+e.FirstURL)
	}
	return strings.Join(parts, "; ")
}

func (e *GitHubRateLimitError) Summary() string {
	if e == nil {
		return ""
	}

	parts := []string{e.summaryPrefix()}
	if message := normalizeGitHubRateLimitMessage(e.Message); message != "" {
		parts = append(parts, message)
	}
	if detail := e.ResetSummary(); detail != "" {
		parts = append(parts, detail)
	}
	return strings.Join(parts, "; ")
}

func (e *GitHubRateLimitError) ResetSummary() string {
	if e == nil {
		return ""
	}
	if e.ResetAt.IsZero() {
		return "reset time unknown"
	}

	remaining := time.Until(e.ResetAt)
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("resets in %s at %s", humanizeDurationCompact(remaining), formatLocalResetTime(e.ResetAt))
}

func (e *GitHubRateLimitError) summaryPrefix() string {
	if e.ShortCircuited {
		return "GitHub API rate limit still active (not a timeout)"
	}
	return "GitHub API rate limit reached (not a timeout)"
}

func AsGitHubRateLimitError(err error) (*GitHubRateLimitError, bool) {
	var rateErr *GitHubRateLimitError
	if !errors.As(err, &rateErr) {
		return nil, false
	}
	return rateErr, true
}

type githubRateLimitWindow struct {
	mu         sync.RWMutex
	statusCode int
	status     string
	remaining  string
	message    string
	resetAt    time.Time
	firstURL   string
}

var githubAPIRateLimitWindow githubRateLimitWindow

func activeGitHubRateLimit(requestURL string) *GitHubRateLimitError {
	if !isGitHubAPIURL(requestURL) {
		return nil
	}

	now := time.Now()
	githubAPIRateLimitWindow.mu.RLock()
	statusCode := githubAPIRateLimitWindow.statusCode
	status := githubAPIRateLimitWindow.status
	remaining := githubAPIRateLimitWindow.remaining
	message := githubAPIRateLimitWindow.message
	resetAt := githubAPIRateLimitWindow.resetAt
	firstURL := githubAPIRateLimitWindow.firstURL
	githubAPIRateLimitWindow.mu.RUnlock()

	if resetAt.IsZero() {
		return nil
	}
	if !now.Before(resetAt) {
		clearGitHubRateLimitWindow()
		return nil
	}

	return &GitHubRateLimitError{
		URL:            requestURL,
		StatusCode:     statusCode,
		Status:         status,
		Remaining:      remaining,
		Message:        message,
		ResetAt:        resetAt,
		FirstURL:       firstURL,
		ShortCircuited: true,
	}
}

func rememberGitHubRateLimit(err *GitHubRateLimitError) {
	if err == nil || err.ResetAt.IsZero() {
		return
	}

	now := time.Now()
	githubAPIRateLimitWindow.mu.Lock()
	defer githubAPIRateLimitWindow.mu.Unlock()

	if githubAPIRateLimitWindow.resetAt.After(now) && !err.ResetAt.After(githubAPIRateLimitWindow.resetAt) {
		return
	}

	githubAPIRateLimitWindow.statusCode = err.StatusCode
	githubAPIRateLimitWindow.status = err.Status
	githubAPIRateLimitWindow.remaining = err.Remaining
	githubAPIRateLimitWindow.message = err.Message
	githubAPIRateLimitWindow.resetAt = err.ResetAt
	githubAPIRateLimitWindow.firstURL = err.URL
}

func clearGitHubRateLimitWindow() {
	githubAPIRateLimitWindow.mu.Lock()
	defer githubAPIRateLimitWindow.mu.Unlock()

	githubAPIRateLimitWindow.statusCode = 0
	githubAPIRateLimitWindow.status = ""
	githubAPIRateLimitWindow.remaining = ""
	githubAPIRateLimitWindow.message = ""
	githubAPIRateLimitWindow.resetAt = time.Time{}
	githubAPIRateLimitWindow.firstURL = ""
}

func githubRateLimitErrorFromResponse(resp *http.Response, requestURL string, body []byte) *GitHubRateLimitError {
	if resp == nil || !isGitHubAPIURL(requestURL) {
		return nil
	}

	message := githubAPIErrorMessage(body)
	remaining := strings.TrimSpace(resp.Header.Get("X-RateLimit-Remaining"))
	if !isGitHubRateLimitedResponse(resp.StatusCode, remaining, message) {
		return nil
	}

	status := strings.TrimSpace(resp.Status)
	if status == "" {
		status = fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return &GitHubRateLimitError{
		URL:        requestURL,
		StatusCode: resp.StatusCode,
		Status:     status,
		Remaining:  remaining,
		Message:    message,
		ResetAt:    parseGitHubRateLimitReset(resp, time.Now()),
		FirstURL:   requestURL,
	}
}

func isGitHubAPIURL(raw string) bool {
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return strings.Contains(strings.ToLower(raw), "api.github.com/")
	}
	return strings.EqualFold(parsed.Host, "api.github.com")
}

func parseGitHubRateLimitReset(resp *http.Response, now time.Time) time.Time {
	if resp == nil {
		return time.Time{}
	}

	if raw := strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset")); raw != "" {
		if unixSeconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return time.Unix(unixSeconds, 0).UTC()
		}
	}

	if raw := strings.TrimSpace(resp.Header.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil {
			return now.Add(time.Duration(seconds) * time.Second)
		}
		if retryAt, err := http.ParseTime(raw); err == nil {
			return retryAt.UTC()
		}
	}

	return time.Time{}
}

func githubAPIErrorMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		return strings.TrimSpace(payload.Message)
	}

	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if len(trimmed) > 180 {
		trimmed = trimmed[:177] + "..."
	}
	return trimmed
}

func isGitHubRateLimitedResponse(statusCode int, remaining, message string) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode != http.StatusForbidden {
		return false
	}
	if strings.TrimSpace(remaining) == "0" {
		return true
	}

	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "rate limit")
}

func normalizeGitHubRateLimitMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}

	lower := strings.ToLower(message)
	if strings.Contains(lower, "secondary rate limit") {
		return message
	}
	if strings.Contains(lower, "rate limit") {
		return ""
	}
	return message
}

func humanizeDurationCompact(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	d = d.Round(time.Second)
	if d < time.Second {
		return "1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		minutes := int(d / time.Minute)
		seconds := int((d % time.Minute) / time.Second)
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	if d < 24*time.Hour {
		hours := int(d / time.Hour)
		minutes := int((d % time.Hour) / time.Minute)
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}

	days := int(d / (24 * time.Hour))
	hours := int((d % (24 * time.Hour)) / time.Hour)
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

func formatLocalResetTime(resetAt time.Time) string {
	localReset := resetAt.Local()
	localNow := time.Now().In(localReset.Location())
	layout := "3:04PM MST"
	if localReset.Year() != localNow.Year() || localReset.YearDay() != localNow.YearDay() {
		layout = "2006-01-02 3:04PM MST"
	}
	return localReset.Format(layout)
}

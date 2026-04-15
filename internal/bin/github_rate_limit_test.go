package bin

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGitHubRateLimitErrorFromResponseDetectsResetWindow(t *testing.T) {
	clearGitHubRateLimitWindow()
	t.Cleanup(clearGitHubRateLimitWindow)

	resetAt := time.Now().Add(37 * time.Minute)
	header := make(http.Header)
	header.Set("X-RateLimit-Remaining", "0")
	header.Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Status:     "403 Forbidden",
		Header:     header,
	}

	err := githubRateLimitErrorFromResponse(resp, "https://api.github.com/repos/example/repo/releases/latest", []byte(`{"message":"API rate limit exceeded for 127.0.0.1."}`))
	if err == nil {
		t.Fatal("expected a GitHub rate-limit error")
	}
	if err.ResetAt.IsZero() {
		t.Fatal("expected reset time to be parsed")
	}
	if !strings.Contains(err.Summary(), "GitHub API rate limit reached") {
		t.Fatalf("expected summary to mention GitHub API rate limiting, got %q", err.Summary())
	}
	if !strings.Contains(err.ResetSummary(), "resets in") {
		t.Fatalf("expected reset summary to contain a countdown, got %q", err.ResetSummary())
	}
}

func TestRememberGitHubRateLimitShortCircuitsFurtherGitHubAPIRequests(t *testing.T) {
	clearGitHubRateLimitWindow()
	t.Cleanup(clearGitHubRateLimitWindow)

	firstURL := "https://api.github.com/repos/example/one/releases/latest"
	rememberGitHubRateLimit(&GitHubRateLimitError{
		URL:        firstURL,
		StatusCode: http.StatusForbidden,
		Status:     "403 Forbidden",
		ResetAt:    time.Now().Add(2 * time.Minute),
	})

	err := activeGitHubRateLimit("https://api.github.com/repos/example/two/releases/latest")
	if err == nil {
		t.Fatal("expected later GitHub API request to fail fast")
	}
	if !err.ShortCircuited {
		t.Fatal("expected short-circuited flag to be set")
	}
	if err.FirstURL != firstURL {
		t.Fatalf("expected first URL %q, got %q", firstURL, err.FirstURL)
	}
	if !strings.Contains(err.Error(), "before fetching") {
		t.Fatalf("expected error to explain the request was blocked before fetching, got %q", err.Error())
	}
	if activeGitHubRateLimit("https://raw.githubusercontent.com/example/repo/main/Cargo.toml") != nil {
		t.Fatal("did not expect raw.githubusercontent.com requests to be short-circuited")
	}
}

func TestAsGitHubRateLimitErrorFindsWrappedError(t *testing.T) {
	base := &GitHubRateLimitError{
		URL:     "https://api.github.com/repos/example/repo/releases/latest",
		ResetAt: time.Now().Add(time.Minute),
	}

	wrapped := fmt.Errorf("outer: %w", base)
	got, ok := AsGitHubRateLimitError(wrapped)
	if !ok {
		t.Fatal("expected wrapped rate-limit error to be discovered")
	}
	if got != base {
		t.Fatalf("expected the original rate-limit error pointer, got %#v", got)
	}
}

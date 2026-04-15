package main

import (
	"fmt"
	"os"
	"strings"

	binpkg "github.com/pun/vex/internal/bin"
)

type githubRateLimitFailureCollector struct {
	err   *binpkg.GitHubRateLimitError
	tools []string
}

func (c *githubRateLimitFailureCollector) Record(tool string, err error) bool {
	rateLimitErr, ok := binpkg.AsGitHubRateLimitError(err)
	if !ok {
		return false
	}

	c.tools = append(c.tools, tool)
	if c.err == nil {
		c.err = rateLimitErr
		fmt.Fprintf(os.Stderr, "✗ %s\n", rateLimitErr.Summary())
		if !rateLimitErr.ResetAt.IsZero() {
			fmt.Fprintln(os.Stderr, "  further GitHub API requests in this run will fail fast until the reset time")
		}
	}
	return true
}

func (c *githubRateLimitFailureCollector) PrintSummary() {
	if c.err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "  affected tools: %s\n", strings.Join(c.tools, ", "))
	if c.err.FirstURL != "" {
		fmt.Fprintf(os.Stderr, "  first blocked request: %s\n", c.err.FirstURL)
	}
}

func (c *githubRateLimitFailureCollector) SummarySuffix() string {
	if c.err == nil {
		return ""
	}
	return " (GitHub API rate limited)"
}

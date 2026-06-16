package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// git subject-length convention: ≤50 ideal, ≤72 before hard wrap.
const (
	subjectIdeal = 50
	subjectMax   = 72
)

// commitLevel grades a subject line's length.
type commitLevel int

const (
	levelOK   commitLevel = iota // ≤ subjectIdeal
	levelWarn                    // subjectIdeal+1 .. subjectMax
	levelOver                    // > subjectMax
)

// subjectLevel grades a subject of n runes.
func subjectLevel(n int) commitLevel {
	switch {
	case n > subjectMax:
		return levelOver
	case n > subjectIdeal:
		return levelWarn
	default:
		return levelOK
	}
}

// counterStyle maps a subject level to its advisory color.
func counterStyle(l commitLevel) lipgloss.Style {
	switch l {
	case levelOver:
		return delStyle
	case levelWarn:
		return warnStyle
	default:
		return addStyle
	}
}

// buildCommitMessage joins subject and body git-style: a blank line separates
// them, and the body is omitted entirely when empty.
func buildCommitMessage(subject, body string) string {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if body == "" {
		return subject
	}
	return subject + "\n\n" + body
}

// splitCommitMessage splits a full commit message into its subject (first line)
// and body (everything after), mirroring git's own model.
func splitCommitMessage(full string) (subject, body string) {
	full = strings.ReplaceAll(full, "\r\n", "\n")
	lines := strings.SplitN(full, "\n", 2)
	subject = strings.TrimSpace(lines[0])
	if len(lines) == 2 {
		body = strings.TrimSpace(lines[1])
	}
	return subject, body
}

// noticeText is the transient success line shown after a commit lands.
func noticeText(hash, subject string) string {
	hash = strings.TrimSpace(hash)
	subject = strings.TrimSpace(subject)
	if hash == "" {
		return "Committed " + subject
	}
	return "Committed " + hash + " " + subject
}

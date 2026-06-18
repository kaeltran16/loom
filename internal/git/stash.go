package git

import (
	"strconv"
	"strings"
)

// Stash is one entry from `git stash list`.
type Stash struct {
	Ref     string // stash@{0}
	Index   int
	Message string
	Branch  string
	Age     string
}

func parseStashes(out []byte) []Stash {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	stashes := make([]Stash, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) != 3 {
			continue
		}
		idx, ok := stashIndex(parts[0])
		if !ok {
			continue
		}
		msg := parts[1]
		stashes = append(stashes, Stash{
			Ref:     parts[0],
			Index:   idx,
			Message: msg,
			Branch:  stashBranchFromSubject(msg),
			Age:     parts[2],
		})
	}
	return stashes
}

func stashIndex(ref string) (int, bool) {
	if !strings.HasPrefix(ref, "stash@{") || !strings.HasSuffix(ref, "}") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(ref, "stash@{"), "}"))
	return n, err == nil
}

func stashBranchFromSubject(subject string) string {
	for _, prefix := range []string{"WIP on ", "On "} {
		if rest, ok := strings.CutPrefix(subject, prefix); ok {
			branch, _, ok := strings.Cut(rest, ":")
			if ok {
				return strings.TrimSpace(branch)
			}
		}
	}
	return ""
}

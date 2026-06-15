package git

import (
	"bufio"
	"bytes"
	"strings"
)

// Branch is a local branch and its upstream relationship.
type Branch struct {
	Name     string
	Upstream string
	Current  bool
}

func parseBranches(out []byte) []Branch {
	var branches []Branch
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x00")
		if len(fields) < 3 {
			continue
		}
		branches = append(branches, Branch{
			Name:     fields[0],
			Upstream: fields[1],
			Current:  fields[2] == "*",
		})
	}
	return branches
}

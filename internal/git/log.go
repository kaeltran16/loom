package git

import (
	"bufio"
	"bytes"
	"strings"
)

// Commit is one entry from git log.
type Commit struct {
	Hash    string
	Subject string
	Author  string
	RelTime string
}

func parseLog(out []byte) []Commit {
	var commits []Commit
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) < 4 {
			continue
		}
		commits = append(commits, Commit{Hash: f[0], Subject: f[1], Author: f[2], RelTime: f[3]})
	}
	return commits
}

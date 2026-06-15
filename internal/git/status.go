package git

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// FileStatus is one changed path in the working tree.
type FileStatus struct {
	Path      string
	Staged    rune // index status char (X); '.' means none
	Worktree  rune // worktree status char (Y); '.' means none
	Untracked bool
	Unmerged  bool
}

// IsStaged reports whether the index differs from HEAD for this path.
func (f FileStatus) IsStaged() bool { return f.Staged != '.' && f.Staged != 0 }

// BranchInfo is the current branch and its relationship to upstream.
type BranchInfo struct {
	Name     string
	Upstream string
	Ahead    int
	Behind   int
}

// parseStatus parses `git status --porcelain=v2 --branch` output.
func parseStatus(out []byte) ([]FileStatus, BranchInfo, error) {
	var files []FileStatus
	var br BranchInfo

	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			br.Name = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.upstream "):
			br.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			parts := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(parts) == 2 {
				br.Ahead, _ = strconv.Atoi(strings.TrimPrefix(parts[0], "+"))
				br.Behind, _ = strconv.Atoi(strings.TrimPrefix(parts[1], "-"))
			}
		case strings.HasPrefix(line, "1 "):
			// 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			f := strings.Fields(line)
			if len(f) >= 9 {
				files = append(files, fileFromXY(f[1], strings.Join(f[8:], " ")))
			}
		case strings.HasPrefix(line, "2 "):
			// 2 <XY> ... <Xscore> <path>\t<orig>
			f := strings.SplitN(line, " ", 10)
			if len(f) >= 10 {
				path := f[9]
				if i := strings.IndexByte(path, '\t'); i >= 0 {
					path = path[:i] // keep the new path
				}
				files = append(files, fileFromXY(f[1], path))
			}
		case strings.HasPrefix(line, "u "):
			// u <xy> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
			f := strings.Fields(line)
			if len(f) >= 11 {
				files = append(files, FileStatus{Path: strings.Join(f[10:], " "), Unmerged: true})
			}
		case strings.HasPrefix(line, "? "):
			files = append(files, FileStatus{Path: line[2:], Untracked: true})
		}
	}
	return files, br, sc.Err()
}

func fileFromXY(xy, path string) FileStatus {
	fs := FileStatus{Path: path}
	if len(xy) == 2 {
		fs.Staged = rune(xy[0])
		fs.Worktree = rune(xy[1])
	}
	return fs
}

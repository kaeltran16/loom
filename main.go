// Command loom is a terminal UI git client.
package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kael02/loom/internal/git"
	"github.com/kael02/loom/internal/ui"
)

func main() {
	ctx := context.Background()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom: cannot determine working directory:", err)
		os.Exit(1)
	}

	repo, err := git.Open(ctx, wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		fmt.Fprintln(os.Stderr, "run loom from inside a git repository.")
		os.Exit(1)
	}

	p := tea.NewProgram(ui.NewModel(ctx, repo), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

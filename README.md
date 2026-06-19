# loom

A keyboard-driven terminal git client. loom shells out to your real `git`, so it uses your
existing credentials, hooks, and config.

## Build

Requires Go 1.22+ and `git` on PATH.

```
go build -o loom.exe .
```

## Run

From inside any git repository:

```
loom
```

loom is read-only until you act: nothing is staged, committed, or pushed without an explicit keystroke.

## Keys

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` / `Tab` | focus Files / Branches / Commits / Stashes |
| `j` `k` / `↑` `↓` | move cursor |
| `space` | stage / unstage file from Files, or mark / unmark commit from Commits |
| `d` | discard changes (confirm with `y`) |
| `s` | save current work as a stash from Files or Stashes |
| `a` `o` `d` | apply / pop / drop selected stash |
| `enter` | switch to the selected branch |
| `y` | cherry-pick marked commits from Commits after confirmation |
| `/` | open commit search from Commits |
| `Tab` / `j` `k` / `Enter` / `Esc` | search mode: switch fields / choose branch or author / apply / cancel |
| `c` | commit (type message, `Ctrl-D` to confirm, `Esc` to cancel) |
| `C` | amend last commit (edit message, `Ctrl-D` to confirm) |
| `f` `p` `P` | fetch / pull / push |
| `esc` | cancel a running fetch / pull / push |
| `?` | help · `q` quit |

## Terminal notes (Windows)

Use **Windows Terminal** (not the legacy console host) for truecolor and proper box-drawing.
Branch/arrow glyphs render best with a Nerd Font; loom falls back to plain ASCII markers
(`+ ? !`) where glyphs are unavailable.

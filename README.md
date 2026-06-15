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
| `1` `2` `3` / `Tab` | focus Files / Branches / Commits |
| `j` `k` / `↑` `↓` | move cursor |
| `space` | stage / unstage the selected file |
| `d` | discard changes (confirm with `y`) |
| `enter` | switch to the selected branch |
| `c` | commit (type message, `Ctrl-D` to confirm, `Esc` to cancel) |
| `f` `p` `P` | fetch / pull / push |
| `?` | help · `q` quit |

## Terminal notes (Windows)

Use **Windows Terminal** (not the legacy console host) for truecolor and proper box-drawing.
Branch/arrow glyphs render best with a Nerd Font; loom falls back to plain ASCII markers
(`+ ? !`) where glyphs are unavailable.

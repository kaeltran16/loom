# loom — UI Color & Selection Design Spec

**Date:** 2026-06-12
**Status:** Approved (design), implementation pending
**Location:** `C:\Users\kael02\IdeaProjects\loom`
**Scope:** `internal/ui/panels.go`, `internal/ui/view.go` (styling only)

## 1. Problem

The UI reads as plain, and the selected row in the focused list is hard to spot.

- **Weak selection.** The selected row uses `cursorStyle` (background ANSI `24`, foreground `15`), but `cursorStyle.Render(line)` colors only the *text width* — the highlight is a ragged stub, not a row-spanning bar — and dark-blue-on-near-black is low contrast.
- **Monochrome chrome.** Color today is confined to file-status markers and the diff body. The top bar, workflow tabs, panel titles, status-rail section headers, and footer all render in the default terminal foreground, so nothing establishes hierarchy.

## 2. Goals / Non-goals

**Goals**
- Make the selected row unmistakable (full-width, high-contrast bar).
- Add enough color to the chrome to establish hierarchy (focus, active tab, status), without noise.

**Non-goals**
- No new model state, no behavior or key-handling changes.
- No new files.
- No theming system / configurability (YAGNI — single accent, hard-coded like the rest of the palette).
- Diff-body coloring is already adequate and stays untouched.

## 3. Guiding principle: text plain, color at the render boundary

Color uses meaning, not decoration: it encodes focus, selection, and status. A git TUI is stared at for long stretches, so the chrome gets just enough accent for hierarchy and the *selection* is the only deliberately bold element.

The implementation rule that keeps the test suite green by construction:

> **Text-producing functions return plain strings; color is applied only where they are rendered.**

This matters because the tests split in two:

| Assertion kind | Examples | Color-safe? |
|---|---|---|
| `strings.Contains` | most View/panel/rail tests | Yes — an escape-wrapped `"…Ready…"` still *contains* `"Ready"` |
| `==` (exact) | `commandStateText`, `footerActions`, `panelTitle` | No — would break if colored inside |

So `commandStateText()`, `footerActions()`, and `panelTitle()` stay plain (single source of the *words*); styling wraps their output at the call site.

## 4. Palette

All styles already live in one `var` block in `panels.go` — the single source of truth. One accent is added by **reusing the existing cyan (ANSI `14`)** already used by the caret and diff-hunk lines, promoted to a named accent. No new hues.

| Role | Color | Notes |
|---|---|---|
| Accent (focus, active tab, headers, caret) | ANSI `14` cyan | already in palette |
| Selection bar background | ANSI `31` (brighter than current `24`) + bold, fg `15` | the one bold element |
| Status: ready / working / error | green `10` / yellow `11` / red `9` | reuse existing status colors |
| Muted (inactive tabs, descriptions) | gray `245` | already in palette |

## 5. Changes by region

1. **Selection bar (`panels.go`, the core fix).** Thread the panel's content width into `styledPanelLines`. Render the selected row as `cursorStyle.Width(barW).Render(line)` (where `barW = contentW − width(caret)`) so the highlight fills the row. Brighten background `24 → 31`, add bold; keep the cyan `▌` caret. Non-selected rows unchanged.

2. **Focus cue (`panels.go`).** Focused panel border goes cyan (in `renderPanel`); focused panel title wrapped in the accent at render time. `panelTitle()` itself stays plain.

3. **Top bar (`view.go`).** At render time in `topBar()` / `workflowTab()`: active tab cyan (keeps its `[…]` brackets), inactive tabs muted, branch name white, ahead/behind green, state word colored. `commandStateText()` keeps returning plain text.

4. **Status rail (`view.go`).** Color the section headers (`Workflow`, `Command`, `Recent`, `Selected`) in the accent and the state word — **text and casing unchanged** (tests assert `Contains("Workflow")`, so no small-caps).

5. **Footer (`view.go`) — lowest priority, optional.** To accent key letters without breaking the `==` test, refactor `footerActions()` to build from a small `[]keyHint{key, desc}` slice: the plain join reproduces the *exact* current strings (existing tests untouched), and a parallel styled join colors the keys. May be cut to keep churn minimal.

6. **Diff body.** Unchanged.

**Smallest viable cut:** regions 1–2 fix the literal complaint (selection + focus). 3–5 are the "more color" polish. Planned delivery: 1–4, with 5 optional.

## 6. Testing strategy

- **Existing tests stay green.** `Contains`-based assertions are color-safe; the three `==` functions stay plain by the §3 rule. Width/height-fitting tests still apply (styling adds no layout beyond the selection bar's intended full-width padding).
- **Updated:** the two `styledPanelLines` tests gain the new width argument.
- **Added:** one test asserting the selected row fills the panel — `lipgloss.Width(selectedRow) == contentW` — verifying the full-width bar at the logic boundary, not by inspecting escape codes.
- No test asserts on raw ANSI sequences; behavior is verified through visible text and cell widths.

## 7. Risks

- **Color profile in CI/non-TTY.** lipgloss may strip color when output isn't a TTY; this is irrelevant to correctness here because no test inspects escape codes and `lipgloss.Width` counts visible cells regardless.
- **Shade tuning.** Exact selection background (`31`) and accent intensity are cheap to adjust after seeing them in a real terminal; they are isolated to the `var` block.

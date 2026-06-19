# Loom Commit Search Design

Date: 2026-06-19
Status: approved concept, pending user review of written spec
Scope: add a dedicated commit search workflow to Loom's existing Commits panel.

## Goal

Make the Commits workflow useful for investigation, not only recent history browsing.

The user should be able to:

- open a dedicated search form from the Commits workflow;
- search commits by message text;
- choose the branch to search;
- choose an author filter;
- apply the search and browse matching commits in the existing Commits list;
- clear the search and return to normal recent history.

This is a read-only workflow. It must not mutate repository state.

## Current Context

Loom is a keyboard-driven terminal Git client with existing top-level workflows for Files,
Branches, Commits, and Stashes. The Commits workflow currently loads a recent log and previews
the selected commit in the main pane with the existing `git show` path.

Commit search should extend that workflow instead of adding a new top-level panel. Search
results are still commits, so they belong in the Commits list and should reuse the current
commit preview behavior.

## Product Shape

Press `/` while focused on Commits to open a dedicated Commit Search mode in the main pane.

The search form has three fields:

1. `Query`: free text searched against commit messages.
2. `Branch`: selected branch or ref to search, defaulting to the current branch.
3. `Author`: selected author or `Any`, defaulting to `Any`.

Keyboard behavior:

| Key | Action |
|---|---|
| `/` | open Commit Search from Commits |
| `Tab` / `Shift+Tab` | move between Query, Branch, and Author |
| `j` / `k` or `up` / `down` | move inside Branch or Author selector |
| `Enter` | apply search |
| `Esc` | cancel if editing, or clear the active search if results are already applied |

After applying search:

- the Commits list shows matching results;
- the selected result still previews through the existing main-pane commit detail;
- the top bar or status rail shows the active filter summary, for example:
  `Search: "fix auth" | branch main | author Kael`;
- clearing the search reloads the normal recent commit list for the current branch.

## Non-Goals

- No command-language parser in v1.
- No `author:`, `file:`, `since:`, or `before:` query syntax in v1.
- No commit body full-text indexing outside what Git can provide.
- No path-scoped file history in this pass.
- No branch compare in this pass.
- No cherry-pick, checkout, reset, or other commit mutations.
- No multi-select result actions.

## Git Layer

Add typed commit search support to `internal/git`. The UI must call repo methods and never
build raw Git command strings itself.

New query type:

```go
type CommitSearch struct {
    Query  string
    Ref    string
    Author string
    Limit  int
}
```

New repo methods:

```go
SearchCommits(ctx context.Context, q CommitSearch) ([]Commit, error)
CommitAuthors(ctx context.Context, ref string, limit int) ([]string, error)
```

Expected Git commands:

- `git log --format=%H%x00%s%x00%an%x00%ar -n <limit> <ref> --grep=<query>`
- `git log --format=%an -n <limit> <ref>` for author discovery

If `Query` is empty, `SearchCommits` should behave like a branch-scoped log with optional
author filtering. If `Author` is empty or `Any`, no author filter is applied.

Hash search should be best-effort in v1. Git `--grep` searches commit messages, not hashes.
If the query looks like a short hash, the UI may filter loaded results by hash, but v1 does
not need a separate `git show <hash>` search path.

Author discovery should dedupe names, sort them for stable display, and prepend `Any` in the
UI. Discovery can be limited to recent history to keep it cheap.

## UI Workflow

Add a new mode:

```go
ModeCommitSearch
```

Model additions:

```go
commitSearch commitSearchState
authors      []string
```

Suggested state:

```go
type commitSearchState struct {
    Query        string
    Branch       string
    Author       string
    Field        commitSearchField
    Active       bool
    Summary      string
    BranchCursor int
    AuthorCursor int
}
```

The Branch selector should reuse the existing loaded branch list. If branch data is missing,
the current branch from `BranchInfo` is enough for v1.

The Author selector should load when opening search mode or when the branch changes. It is
acceptable for v1 to show `Any` until authors finish loading.

Search results should replace `m.commits`, not create a separate result collection. The active
search metadata in `commitSearch` is what tells the view that the list is filtered.

## Data Flow

Opening search:

```text
[/] on Commits
  -> mode = ModeCommitSearch
  -> initialize Query, Branch, Author from current state
  -> loadCommitAuthors(branch)
  -> render search form in main pane
```

Applying search:

```text
[Enter] in ModeCommitSearch
  -> dispatch searchCommits(query, branch, author)
  -> busy = true
  -> commitsLoadedMsg replaces m.commits with matching commits
  -> commitSearch.Active = true
  -> mode = Normal
  -> main pane previews selected result
```

Clearing search:

```text
[Esc] while active search is applied
  -> commitSearch.Active = false
  -> reload normal commits for current branch
```

## Error Handling

- Git-layer methods wrap stderr with command context.
- Empty search is allowed and behaves as branch plus optional author filtering.
- If search returns no commits, the Commits panel should show an empty-state row such as
  `No commits match search`.
- Author loading failure should not block search. Show `Any` only and surface the error in the
  footer using existing conventions.
- Branch changes while search is open should keep Query intact and reset Author to `Any`
  unless the selected author exists in the newly loaded author list.

## Testing

Add focused tests around Git command construction, reducer state, and rendering:

- `SearchCommits` dispatches `git log` with ref, grep, author, format, and limit.
- `SearchCommits` without query omits `--grep`.
- `CommitAuthors` dedupes and sorts names.
- `/` only opens search mode from Commits.
- `Tab` cycles search fields.
- branch and author selectors move independently.
- `Enter` dispatches search and returns to normal mode.
- active search summary renders in top/status context.
- `Esc` cancels editing search mode.
- `Esc` clears an active applied search and reloads normal commits.
- empty search result renders a clear empty state.

Prefer reducer and helper tests. Avoid brittle full-screen ANSI snapshots.

## Acceptance Criteria

- Pressing `/` from Commits opens a dedicated Commit Search form.
- The form has Query, Branch, and Author controls.
- Search runs through the git layer, not raw UI command strings.
- Results replace the Commits list and keep existing commit preview behavior.
- Active search state is visible in the UI.
- Search can be cleared to restore normal recent commits.
- The feature is read-only and does not affect Files, Branches, Stashes, commit, amend, remote,
  or conflict workflows.

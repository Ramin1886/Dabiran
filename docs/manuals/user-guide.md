# User Manual

Audience: everyone who works on the canvas — team owners and team members.
No installation is required; you only need the platform URL from your
administrator and a WebGL-capable browser (any current Chrome, Firefox,
Edge, or Safari).

## 1. What You're Looking At

The platform draws your repository's history as a graph on an infinite
canvas:

* **Time flows left → right.** A commit's horizontal position encodes when
  it was authored; the oldest commit sits at the origin.
* **Branches are horizontal lanes.** A commit stays in its parent's lane;
  new branches open new lanes below.
* **Curved connectors are splits and merges.** A line bending between lanes
  means a branch was created (split) or merged back.
* **Labels** show a commit's tag when it has one (e.g. `v1.0.0`), otherwise
  its short hash.

Everyone viewing the same repository map shares the canvas in real time: you
see your teammates' cursors move, and drawings appear for everyone
instantly.

## 2. Signing In

Open the platform URL. Depending on how your administrator configured the
instance you are either signed in automatically (development login) or sent
through **GitHub sign-in** — authorize the application and you'll return to
the canvas.

The status line at the top right tells you what's happening:
`Authenticating… → Loading topology… → Loaded N commits`. If it shows
`Error: …`, see Troubleshooting (Section 7).

## 3. Navigating the Canvas

| Action | How |
| :--- | :--- |
| **Pan** | Click and drag anywhere on empty canvas |
| **Zoom** | Mouse wheel / trackpad scroll — zoom is anchored at your pointer, so the spot under your cursor stays put |
| **Inspect a commit** | Click its node |

There are no edges to hit — the canvas is infinite. If you lose the graph,
zoom out until it reappears.

## 4. Inspecting Commits

Clicking a node highlights it and opens the **commit panel** on the right:

| Field | Meaning |
| :--- | :--- |
| Short hash & tag | The commit's identity; a cyan tag badge appears when the commit is tagged |
| Message | Full commit message |
| Author / Date | Who authored it and when |
| Lineage hash keys | The parent commit ids — two parents means a merge commit; "Origin Trajectory" marks the repository's first commit |

Click a different node to switch the panel; it follows your selection.

## 5. Searching & Filtering

Type in the **Search commits…** field in the top bar. The graph filters
instantly — a commit stays visible when your text matches its:

* hash,
* author name, or
* commit message (all case-insensitive).

**Structural context is always preserved:** split points (commits where a
branch was created) and merge commits remain visible even when they don't
match, so the filtered graph keeps its shape and you can always trace where
a matching commit came from and where it went. Clear the field to restore
the full graph.

## 6. Collaborating

### Live cursors

Teammates viewing the same repository map appear as colored cursor dots that
move in real time. Cursors are anchored to the *graph*, not the screen — a
teammate pointing at a commit points at it for you too, regardless of your
zoom level.

### Drawing dependency lines

Use drawings to mark relationships the Git history can't express — "this
commit fixes what that one broke", cross-repository dependencies, areas to
discuss in review.

1. Click **Draw** in the top bar (it lights up cyan: `Drawing: ON`; your
   cursor becomes a crosshair).
2. Press and drag from the start point to the end point. Release to commit
   the line.
3. The line appears for **everyone in the room** and stays anchored to the
   canvas at any zoom level.
4. Click the button again to leave drawing mode (panning is disabled while
   it's on).

> **Persistence note:** drawings are shared live and survive as long as at
> least one participant keeps the map open. They are not yet stored
> server-side — when the last person leaves the room, the drawing layer
> resets. Server-side persistence is on the roadmap.

### Multi-repository maps

When your team has several repositories on one canvas, commits from all of
them share the same timeline, and node ids are prefixed per repository — so
you can visually correlate work across projects and draw links between them.

## 7. Troubleshooting

| Symptom | What it means | What to do |
| :--- | :--- | :--- |
| Status line: `Error: …` right after loading | The browser can't reach the backend | Refresh once; if it persists, contact your administrator |
| Canvas is empty, status says `Loaded 0 commits` or an error mentions repositories | No repository is provisioned for your workspace yet | Ask your team owner/administrator to register a repository |
| You can't see a teammate's cursor | You're on different repository maps, or one of you lost the connection | Confirm you both opened the same map; refresh |
| Drawings vanished | Everyone left the room since you last looked | Expected in this release (see persistence note above) |
| Graph disappeared while navigating | You panned/zoomed away from it | Zoom out until the graph reappears |
| Page is blank or rendering is garbled | WebGL unavailable (old browser/driver, or disabled) | Update the browser, enable hardware acceleration |

## 8. Quick Reference

| Control | Location | Action |
| :--- | :--- | :--- |
| Drag | canvas | Pan |
| Scroll wheel | canvas | Zoom (pointer-anchored) |
| Click node | canvas | Open commit panel |
| Search field | top bar | Filter commits (splits/merges always retained) |
| **Draw** button | top bar | Toggle annotation drawing mode |
| Status line | top bar, right | Connection / loading state |

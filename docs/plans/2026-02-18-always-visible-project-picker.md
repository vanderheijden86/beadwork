# Always-Visible Project Picker (k9s-style)

**Date:** 2026-02-18
**Epic:** bd-ey3

## Overview

Transform the project picker from a full-screen overlay into an always-visible header that sits above the main content, matching k9s's namespace header pattern.

## Design

### Two display modes

**Expanded** (default on launch):
```
  <enter> Switch  <1-9> Quick  <u> Fav  </>Filter  <esc> Minimize
──────────────────────── projects[4] ────────────────────────────
  #  NAME            PATH              OPEN  READY  BLOCKED
► 1  my-app          ~/dev/my-app        12      5        2
  2  backend         ~/dev/backend        8      3        1
  3  frontend        ~/dev/frontend       5      2        0
═══════════════════════ issues(my-app)[12] ══════════════════════
  (main content: list/tree/board)
```

**Minimized** (press P or Esc):
```
  Project: my-app (5/2/1)  │  <1> my-app  <2> backend  │  <P> Expand
═══════════════════════ issues(my-app)[12] ══════════════════════
  (main content gets more vertical space)
```

### Focus model

- Main content always has keyboard focus (j/k/enter navigate issues)
- Number keys 1-9 quick-switch projects in both modes
- P toggles expanded/minimized
- Expanded picker is display-only (no cursor, no j/k navigation in project rows)
- `/` enters filter mode when expanded (filter input gets temporary focus)

### What changes

1. `showProjectPicker` boolean becomes `pickerExpanded` boolean (picker always present)
2. Project picker removed from overlay if/else chain in View()
3. Picker rendered as part of normal layout: `pickerHeader + body + footer`
4. `renderGlobalHeader()` replaced by picker header (subsumes that role)
5. ProjectPickerModel gets `ViewExpanded()` and `ViewMinimized()` render paths
6. Height calculations: body height = terminal height - picker height - footer height

## Implementation Plan

### Task 1: Add ViewExpanded/ViewMinimized to ProjectPickerModel
- Add `ViewMinimized(activeProject, stats)` method
- Rename existing `View()` to `ViewExpanded()`
- Remove cursor rendering from expanded view (display-only)
- Remove vertical padding/fill from expanded view (parent handles layout)
- Add `PickerHeight()` method that returns rendered line count

### Task 2: Refactor Model.View() layout
- Replace `showProjectPicker` with `pickerExpanded` boolean
- Remove project picker from overlay if/else chain
- Always render picker above body: expanded or minimized
- Remove `renderGlobalHeader()` (picker header replaces it)
- Body height = terminal height - picker height - 1 (footer)
- Pass correct height to all sub-views (list, tree, board, split)

### Task 3: Refactor key handling
- Remove the `if m.showProjectPicker { route all keys to picker }` block
- P key toggles `pickerExpanded` (not `showProjectPicker`)
- Number keys 1-9 always emit SwitchProjectMsg (already works)
- `/` enters picker filter mode when expanded
- Esc during filter exits filter; Esc without filter minimizes picker
- All other keys route to main content as normal

### Task 4: Initialize picker on startup
- Create ProjectPickerModel during model init (not on P press)
- Set `pickerExpanded = true` as default
- Rebuild picker entries on SwitchProjectMsg and window resize
- Keep picker entries updated when issue counts change

### Task 5: Update tests
- Update existing project picker tests for new display-only behavior
- Add tests for expanded/minimized toggle
- Add tests for height calculation with picker
- Update any tests that depend on overlay behavior

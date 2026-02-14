# Narrow Window: Auto-hide Detail Panel

## Problem

When the tree view is opened in a narrow window (width <= 100), the detail panel
is not visible (no room for split view), but `treeDetailHidden` is not set to
`true`. This means Enter toggles expand/collapse instead of showing a full-screen
detail view. The user has no way to see issue details without manually pressing
`d` first.

## Solution

When the window is narrow (`!isSplitView`), automatically set `treeDetailHidden = true`
so that Enter opens the full-screen detail view (existing detail-only mode).

## Behavior

| Window width | Default state | Enter | d | Esc (from detail) |
|---|---|---|---|---|
| <= 100 (narrow) | Tree-only, detail hidden | Full-screen detail | Toggle (same as now) | Back to tree |
| > 100 (wide) | Tree + detail split | Toggle expand | Toggle detail panel | Back to tree |

## Resize behavior

- **Narrow -> wide**: Stay in `treeDetailHidden = true` (manual mode). User
  must press `d` to restore split view. Rationale: don't surprise the user with
  layout changes mid-work.
- **Wide -> narrow**: If detail is visible, auto-hide it (existing behavior at
  line 3468). If detail was already hidden, no change.

## Threshold

Use the existing `SplitViewThreshold` (100). No new constant needed.

## Implementation

In the `tea.WindowSizeMsg` handler, add a branch for `!isSplitView` that sets
`treeDetailHidden = true`. The existing resize handler only manages
`treeDetailHidden` when `isSplitView` is true; the narrow case is unhandled.

```go
// Current code (line 3466-3476):
if m.isSplitView && m.detailPaneWidth() < MinDetailPaneWidth {
    m.treeDetailHidden = true
} else if m.isSplitView {
    m.treeDetailHidden = false
}

// New code:
if !m.isSplitView {
    m.treeDetailHidden = true
    if m.treeViewActive && m.focused == focusDetail {
        m.focused = focusTree
    }
} else if m.detailPaneWidth() < MinDetailPaneWidth {
    m.treeDetailHidden = true
    if m.treeViewActive && m.focused == focusDetail {
        m.focused = focusTree
    }
} else {
    // Wide enough for split: only auto-show if not user-overridden
    // (removed: was unconditionally setting false, conflicting with
    // "stay manual" on resize-up)
}
```

Note: The `else` branch (wide enough) should NOT auto-show the detail panel,
per the "stay manual" requirement. The user presses `d` to restore it.

## Files to change

- `pkg/ui/model.go`: WindowSizeMsg handler (~line 3466)

## Test plan

- Unit test: verify `treeDetailHidden` is true after WindowSizeMsg with width < 100
- Unit test: verify Enter in narrow tree view sets `focused = focusDetail`
- Unit test: verify resize from narrow to wide does NOT auto-show detail

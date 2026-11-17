# DAW Workspace & W01–W12 Spike Record (Phase 0)

> Status: Phase 0 Architecture & Mock UI Verified (`plan.md` 4.3, 5.8, 16.2)

## 1. Window Kernel Decision

- **Custom `WorkspaceController` (`useWorkspaceStore` + `WindowFrame` + `geometry.ts`)** handles overlapping floating windows, z-index focus stack, app-local `pinned` top layer, 4-edge/4-corner 6px resize handles, 8px edge/sibling snapping (`Alt` bypass), `Esc` drag/resize rollback, keyboard move/resize mode, viewport clamping, and offscreen recovery (`W01–W12`).
- **Content Separation (`W06`)**: Draft text and monotonic revisions live in `useDraftBufferStore` keyed by `draftId`, independent of `WindowFrame` mount state so floating <-> docked <-> minimized transitions never lose edits or cursor/buffer state.
- **Docked Host & Tab Groups**: `dockview` is installed in `frontend/package.json` for rich docked split/tab groups; normal floating dragging never forces windows into docked splits (`W05`).

## 2. W01–W12 Verification Matrix

| ID | Behavior | Verification Status |
| --- | --- | --- |
| W01 | Overlapping floating windows & focus/z-order stack | Unit tested (`workspace.test.ts`) + Mock UI |
| W02 | 4-edge/4-corner resize (`300x220` min bounds) | Unit tested (`workspace.test.ts`) |
| W03 | Minimize to WindowBar / Maximize to workspace / Restore / Close | Implemented in `useWorkspaceStore` + `WindowFrame` |
| W04 | 8px snap to edges & siblings + `Alt` bypass + guide lines | Unit tested (`workspace.test.ts`) |
| W05 | Explicit dock toggle separated from floating drag | Implemented in `WindowFrame` + `AppShell` |
| W06 | Floating / docked / minimized preserves `draftId` buffer | Unit tested (`workspace.test.ts`) |
| W07 | Global WindowBar with active top-line & unread badges | Rendered & tested (`App.test.tsx`) |
| W08 | Titlebar double-click maximize/restore | Implemented in `WindowFrame` |
| W09 | Cascade / Tile H / Tile V / Show All / Reset Default / Undo | Unit tested (`workspace.test.ts`) |
| W10 | App-local pinned top layer & close focus fallback | Implemented in `useWorkspaceStore` |
| W11 | `Esc` drag/resize rollback & keyboard move/resize mode | Implemented in `WindowFrame` |
| W12 | Viewport clamping & Recover All Windows | Unit tested (`workspace.test.ts`) |

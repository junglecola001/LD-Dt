# LD Studio

> **Unofficial** Windows desktop client for [Linux.do](https://linux.do) built with Go, Wails v3, and React.

LD Studio brings Linux.do browsing, search, reading, composing, notifications, and bookmarks into a DAW-inspired multi-window workspace on Windows—featuring overlapping floating windows, docking/splits/tabs, persistent layouts, and warm editorial typography.

## Current Project Status

| Verification Channel | Status | Notes |
| --- | --- | --- |
| Local Frontend Checks & Mock UI | Pending | Run via Node/pnpm (`typecheck`, `lint`, `test`) using `DesktopBridge` mock |
| Cloud Go/Wails Build (Actions) | Pending | Windows x64 & ARM64 builds run exclusively on GitHub Actions runners |
| Native Windows Acceptance | Pending | Requires downloading CI artifacts and verifying WebView2, DPI, IME, and auth on Windows |

> **Important:** Per [plan.md](./plan.md), native Go/Wails compilation and packaging are executed on GitHub Actions Windows runners rather than requiring Go/MSVC/Windows SDK locally. Mock UI data is explicitly labeled as demonstration data and does not represent live Linux.do account state.

## Architecture Overview

- **Desktop Host & Application Services (`main.go`, `internal/`)**: Single-process Go backend with Wails v3, shared `RequestCoordinator`, `ResponseClassifier`, `ChallengeCoordinator`, `AuthService`, `TopicService`, `SearchService`, `ComposerService`, `NotificationService`, `BookmarkService`, and `WorkspaceService`.
- **Presentation & DAW Workspace (`frontend/`)**: Single business WebView2 running React + TypeScript, warm paper semantic tokens, `WorkspaceController` (overlapping floating windows + docking + window bar + keyboard navigation), and typed `DesktopBridge` (Wails bindings + deterministic mock adapter).
- **Build & CI (`.github/workflows/`)**:
  - `ci.yml`: Frontend typecheck/lint/unit/mock UI checks and Go `fmt`/`vet`/`test`.
  - `windows-build.yml`: Windows x64 (`amd64`) and `arm64` packaging (NSIS installer + staging ZIP) on hosted Windows runners.
  - `release.yml`: Tag-triggered draft release packaging with checksums.

## Repository Layout

```text
LD-Dt/
├── main.go                    # Wails v3 composition root
├── go.mod                     # Go module & pinned Wails v3 dependency
├── Taskfile.yml               # Wails v3 / Go-Task build orchestration
├── build/                     # Windows NSIS, manifest, and packaging configuration
├── internal/
│   ├── domain/                # Business entities, errors, ports, and invariants (3.4-3.12)
│   ├── app/                   # Application services (Auth, Topic, Search, Composer, Notifications, Workspace)
│   ├── discourse/             # Discourse API adapter & capability contracts
│   ├── network/               # Shared RequestCoordinator, ResponseClassifier, ChallengeCoordinator
│   ├── session/               # Account namespaces, session generation, cookie boundary rules
│   ├── storage/               # SQLite schema/migrations & cache policies
│   └── platform/windows/      # Windows WebView2, credential store, notifications, deep links
├── frontend/                  # React + TypeScript + Vite workspace UI & mock DesktopBridge
├── docs/                      # Business rules, API compatibility, UI specs
├── tests/                     # Cross-layer fixtures and contract scenarios
└── .github/workflows/         # CI, Windows build, and release workflows
```

## License

GNU General Public License v3.0 — see [LICENSE](./LICENSE) and [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md).

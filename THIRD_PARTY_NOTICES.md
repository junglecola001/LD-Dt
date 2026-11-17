# Third-Party Notices and Attribution

LD Studio is licensed under the GNU General Public License v3.0 (GPL-3.0). See [LICENSE](./LICENSE).

## Reference Project

Portions of the behavioral design, request-scheduling state machine, and Discourse compatibility rules in LD Studio were informed by and adapted from:

- **FluxDO** (`Lingyan000/fluxdo`)
  - Repository: <https://github.com/Lingyan000/fluxdo>
  - Reference commit: `185ccaa418dab8ea40192301e39065706192f866` (2026-11-15 11:54:25 UTC)
  - License: GNU General Public License v3.0 (GPL-3.0)
  - Scope of adaptation: Request coordinator / 429 & Cloudflare challenge classification contracts, Discourse feed/topic/draft/upload/notification/bookmark behavioral rules, and edge-case test scenarios. No Dart/Flutter source or Rust binaries were copied directly; Go and TypeScript implementations are clean-room or explicitly documented adaptations with local behavior tests.

## Core Runtime & Framework Dependencies

Exact resolved versions are pinned in `go.mod`, `go.sum`, and `frontend/pnpm-lock.yaml`:

- **Wails v3** (`github.com/wailsapp/wails/v3`, `@wailsio/runtime`) — MIT License
- **React / React DOM** — MIT License
- **Dockview** (`dockview`) — MIT License
- **CodeMirror 6** (`@codemirror/*`) — MIT License
- **TanStack Query / TanStack Virtual** — MIT License
- **Zustand** — MIT License
- **Radix UI Primitives** — MIT License
- **Phosphor Icons** (`@phosphor-icons/react`) — MIT License
- **DOMPurify** (`dompurify`) — Apache-2.0 / MPL-2.0
- **modernc.org/sqlite** — BSD-3-Clause

> Note: Dependency licenses and exact pinned revisions are audited during CI packaging and release preparation.

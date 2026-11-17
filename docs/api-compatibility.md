# Linux.do / Discourse API & Session Compatibility (Phase 0)

> Status: Initial Contract & Verification Checklist (`plan.md` 7.1–7.8, 16.1)
> Note: Live Linux.do verification must ONLY be executed from the downloaded Windows build in a normal user network environment—never from CI or automated probing scripts.

## 1. Planned Verification Items

| Item | Target | Verification Status | Evidence / Notes |
| --- | --- | --- | --- |
| Read-only Discourse endpoints | `/latest.json`, `/top.json`, `/t/{id}.json`, `/search.json` | Mock / Fixture tested | Tested in `internal/discourse/adapter_test.go` against Discourse JSON schema |
| 429 & Cloudflare Classifier | `cf-mitigated`, HTML markers, `Retry-After`, `extras.wait_seconds` | Unit tested | Tested in `internal/network/coordinator_test.go` (7.8 matrix) |
| User API Key / Browser Auth | Supported scopes, nonce/state callback, lifetime | Pending native run | To be verified on downloaded Windows build (`plan.md` 7.3) |
| Business-binding-free Auth Host | Isolated WebView2 window without Go business bindings | Architecture defined | Origin allowlist in `internal/platform/windows/adapter.go`; native WebView2 host pending Phase 0 CI package check |
| Go <-> WebView2 Cookie Boundary | Preserve `Domain`, `Path`, `Expires`, `Secure`, `HttpOnly`, `SameSite`, monotonic `sessionGeneration` | Unit tested | Tested in `internal/session/manager_test.go` |

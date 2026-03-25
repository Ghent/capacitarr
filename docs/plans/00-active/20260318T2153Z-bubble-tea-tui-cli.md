# Bubble Tea TUI/CLI Frontend

> **Status:** ⏸️ Deferred — previous implementation on stale `feature/tui-cli` branch (deleted, recoverable via SHA `57b9d63`)
> **Created:** 2026-03-18
> **Original work:** 21 commits implementing a full terminal UI and CLI for Capacitarr

## Summary

A terminal-based frontend for Capacitarr using [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Go TUI framework) and [Glamour](https://github.com/charmbracelet/glamour) (markdown rendering). Provides both an interactive TUI and non-interactive CLI commands for headless/SSH management.

## What Was Built (on the stale branch)

The `feature/tui-cli` branch contained a near-complete implementation:

| Phase | Feature | Status |
|-------|---------|--------|
| 1 | Foundation — project scaffold, DataProvider interface, SSE client | ✅ Complete |
| 2 | App shell — Bubble Tea app with view switching, keyboard navigation | ✅ Complete |
| 3 | SSE client — real-time event stream consumption in terminal | ✅ Complete |
| 4 | Dashboard view — disk groups, engine status, activity feed | ✅ Complete |
| 5 | Approval queue view — list, approve, reject, snooze | ✅ Complete |
| 6 | Audit log view — history with filtering | ✅ Complete |
| 7 | Charts — ntcharts engine history sparklines | ✅ Complete |
| 8 | Rules view — rule listing and management | ✅ Complete |
| 9 | Settings view — preferences display and editing | ✅ Complete |
| 10 | Integrations view — integration listing and status | ✅ Complete |
| 11 | Help view — Glamour-rendered markdown help | ✅ Complete |
| 12 | Non-interactive CLI — `capacitarr status`, `capacitarr approve <id>`, etc. | ✅ Complete |
| 13 | Build pipeline integration — TUI binary included in release | ✅ Complete |

## Why It Was Deferred

The branch was based on an older version of `main` (before queue management, deletion queue, preview service, and many other features). The API surface changed significantly, making the TUI incompatible with the current backend. Rather than rebasing 21 commits, the decision was made to defer and potentially reimplement against the 2.0 API.

## Architecture

- **DataProvider interface** — abstracts API calls so the TUI can work with both live servers and mock data for testing
- **SSE client** — consumes the same `/api/v1/events` stream as the web frontend for real-time updates
- **View switching** — tab-based navigation between Dashboard, Approval Queue, Audit Log, Rules, Settings, Integrations, Help
- **Non-interactive CLI** — `capacitarr status`, `capacitarr approve <id>`, `capacitarr reject <id>`, `capacitarr run` for scripting and cron jobs

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — TUI styling
- `github.com/charmbracelet/glamour` — Markdown rendering
- `github.com/NimbleMarkets/ntcharts` — Terminal charts (sparklines, history)

## Relationship to 2.0

The TUI is **not part of the 2.0 plan**. It's a separate initiative that could be revisited after 2.0 ships, when the API surface is stable. If reimplemented, it should target the 2.0 API (which includes the new analytics endpoints, per-integration thresholds, and Insights data).

The non-interactive CLI commands (`capacitarr status`, `capacitarr approve`, `capacitarr run`) are independently valuable and could be added to 2.0 without the full TUI — they're thin wrappers around API calls.

## Recovery

The stale branch commits are recoverable via:
```bash
git checkout -b feature/tui-cli-archive 57b9d63
```
This will work until git garbage collects unreachable objects (typically 30-90 days).

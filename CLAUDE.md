# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CZL Browser is a Windows desktop application for managing isolated browser instances with per-instance proxy binding. Built with **Wails v2** (Go backend + React/TypeScript frontend embedded in a WebView2 window), it compiles to a single `czl-browser.exe`.

Key capabilities: multi-profile browser management, proxy pool (HTTP/SOCKS5/Clash/Xray/Sing-Box), Chrome fingerprint spoofing, quick launch (Ctrl+K), backup/restore, OAuth2 login via CZL Connect.

## Build & Development Commands

```bash
# Development (starts Vite dev server + Wails hot-reload)
bat\dev.bat
# Or directly:
wails dev

# Production build → build/bin/czl-browser.exe
bat\build.bat

# Installer packaging (requires NSIS)
bat\publish.bat

# Frontend only (inside frontend/)
npm run dev          # Dev server on port 5218 (via dev-watcher.mjs)
npm run dev:raw      # Raw Vite server without watcher
npm run build        # tsc + vite build → frontend/dist

# Go tests
go test ./backend/...
go test ./backend/internal/...
# Single test
go test ./backend/internal/config/ -run TestConfigLoad
```

## Architecture

### Wails Binding Pattern

The Go `App` struct in `backend/app.go` is bound to the frontend via Wails. All its exported methods become callable from JS via auto-generated bindings in `frontend/src/wailsjs/`. The thin wrapper in `main.go` embeds `backend.App` and hooks into Wails lifecycle (`OnStartup`, `OnShutdown`, `OnBeforeClose`).

Frontend calls Go methods through `window.go.main.App.*`. Backend pushes events to the frontend via `runtime.EventsEmit()` (e.g., `proxy:speed:result`, `browser:instance:crashed`, `app:request-close`).

### Backend Structure (Go)

```
main.go                          # Wails init, window config, system tray
backend/
  app.go                         # Core App struct, all Wails-bound API methods
  app_instance.go                # Browser instance start/stop/restart
  app_backup_ops.go              # Backup/restore/export/import
  app_proxy_import.go            # Clash subscription import
  app_shutdown.go                # Graceful shutdown, service cleanup
  bootstrap.go                   # Config loading with auto-repair
  internal/
    auth/                        # OAuth2 authentication (CZL Connect)
      oauth2.go                  # OAuth2 flow, token exchange, user info
      token_store.go             # Local token persistence
    browser/                     # Browser management
      core.go                    # Chrome version management
      profile.go                 # Profile CRUD
      *_dao.go                   # SQLite data access (ProfileDAO, ProxyDAO, CoreDAO, BookmarkDAO, GroupDAO)
      environment.go             # Launch environment setup
      proxy_speed_scheduler.go   # Periodic proxy health checks
    proxy/                       # Proxy managers
      clash.go                   # Mihomo/Clash integration
      xray.go                    # Xray proxy bridge
      singbox.go                 # Sing-Box proxy bridge
      speedtest.go               # Proxy speed testing
    config/                      # YAML config management
    database/                    # SQLite connection (modernc.org/sqlite)
    logger/                      # Structured JSON logging with memory buffer
    launchcode/                  # Quick launch HTTP API server
    backup/                      # Backup spec and format
    tray/                        # System tray integration
```

**Key pattern**: `backend/app.go` contains all API methods exposed to the frontend. Feature logic lives in `backend/app_*.go` files. Data access uses the DAO pattern in `internal/browser/*_dao.go`. Authentication is handled by `backend/app_auth.go` which wraps `internal/auth/`.

### Frontend Structure (React + TypeScript)

```
frontend/src/
  App.tsx                        # Router, global event listeners, Ctrl+K handler
  modules/
    auth/                        # Login page (OAuth2 guard)
    browser/                     # Browser instance management (largest module)
      api.ts                     # Go backend API wrappers
      pages/                     # Page components
      components/                # Feature UI (modals, filters, tables)
      types.ts                   # TypeScript types
    dashboard/                   # Home/stats page
    settings/                    # App settings
    profile/                     # User profile & license
    charts/                      # Data visualization
  shared/
    components/                  # Reusable UI (Button, Modal, Table, Toast, etc.)
    layout/                      # App layout wrapper
    theme/                       # Theme provider, CSS variables
  store/                         # Zustand stores (notification, backup, layout)
  wailsjs/                       # Auto-generated Wails bindings (do not edit manually)
```

### Data Flow

- **Database**: SQLite (`data/app.db`) via DAO pattern. Auto-migrated on startup.
- **Config**: `config.yaml` at app root. Loaded with auto-repair on missing fields. Supports runtime reload via `ReloadConfig()`.
- **Proxy runtime**: `bin/xray.exe` and `bin/sing-box.exe` shipped in-repo. No external downloads needed.
- **Chrome kernels**: User-provided, placed in `chrome/` directory. Managed through the UI's core management page.
- **Auth**: OAuth2 via CZL Connect (`connect.czl.net`). Tokens stored in `data/auth_token.json`. App requires login to use. User level (groups field: t0-t5, viewer, admin) displayed in Topbar. Auth state managed by `authStore` (Zustand).
- **Profile quota**: Instance create/copy flows are not blocked by `MaxProfileLimit`; quota values are informational only.
- **Quota UI**: Dashboard and Browser List no longer expose redeem/expand quota entry points.

## Key Conventions

- **Language**: All UI text, log messages, and comments are in Chinese
- **Go module**: `czl-browser` (not `czl-browser`)
- **CSS**: Tailwind CSS with CSS variables defined in the theme (no hardcoded Tailwind colors)
- **State management**: Zustand (lightweight stores in `src/store/`)
- **Icons**: Lucide React only (no emoji in UI)
- **Frontend routing**: React Router v6, routes defined in `App.tsx`
- **Backend method naming**: `BrowserProfile*`, `BrowserProxy*`, `BrowserCore*` prefixes for Wails-bound methods
- **Type aliases**: `backend/app.go` re-exports `browser.*` types (e.g., `type BrowserProfile = browser.Profile`) for Wails binding compatibility

## Branches

- `master`: Clean development baseline, no user data
- `user_data`: Includes test `data/app.db` for demo/debugging

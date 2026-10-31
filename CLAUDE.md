# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CZL Browser is a Windows desktop application for managing isolated browser instances with per-instance proxy binding. Built with **Wails v2** (Go backend + React/TypeScript frontend embedded in a WebView2 window), it compiles to a single `czl-browser.exe`.

Key capabilities: multi-profile browser management, proxy pool (HTTP/SOCKS5/Clash/Xray/Sing-Box), Chrome fingerprint spoofing, quick launch (Ctrl+K), backup/restore, OAuth2 login via CZL Connect.

Commercialization and enterprise rollout planning lives in `docs/commercialization/`. When implementing update delivery, workspaces, entitlements, enterprise features, or billing-adjacent flows, align changes with those documents first.

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

### CI (GitHub Actions)

`.github/workflows/build.yml` builds the Windows installer on every push to `main`/`master` (doc-only changes ignored) and on `workflow_dispatch`. Steps mirror `bat\build.bat`/`bat\publish.bat` in PowerShell: install deps → `wails generate module` → `wails build -platform windows/amd64` → assemble staging → NSIS package. The installer **omits the chrome kernel** (gitignored, unavailable in CI); `installer.nsi` makes the `SecChrome` section conditional via `!if /FileExists`, so the same script packages with-kernel locally and without-kernel in CI. Each run uploads the installer as a 90-day artifact and publishes a GitHub Release tagged `v<YYYY.MM.DD>.<run_number>` (date computed in UTC+8 so it matches local; run number disambiguates same-day builds). Each build is a new historical Release, but `make_latest: true` keeps the newest one as GitHub's "Latest" so consumers get a stable pointer to the newest build. The asset is named `CZLBrowser-Setup-<fullVersion>.exe` (version in filename); the alist "GitHub 发行版" proxy (repo is private, proxied via a GitHub token) resolves the latest release's asset through the GitHub API, so a version-stamped filename is fine. Windows-only by design — proxy runtime, NSIS and WebView2 are Windows-specific.

Before binding generation, `build.yml` refreshes the embedded fallback `backend/builtin_cores.json` from the remote manifest (see below). It validates the result with `TestBuiltinCoresManifest` and keeps the repo copy if the fetch or validation fails.

**App update publishing** (end of `build.yml`): `wails build` injects the version with `-ldflags "-X czl-browser/backend.AppVersion=<fullVersion>"`. Local builds stay `dev` and never report an update. czlbrowser is private, so its release assets are not downloadable by users. The installer is therefore also published as a Release of the public `woodchen-ink/czlbrowser-releases`. CZL Connect's release download (slug `czlbrowser`) fronts that repo and is reachable from mainland China: alias `windows` proxies the Release installer (`connect.czl.net/dl/czlbrowser/<tag|latest>/windows`), and alias `update-manifest` proxies the repo's `main/latest.json` file. Connect's own `/dl/czlbrowser/latest.json` is its unsigned release summary, so the client does not use it. The job writes `manifest.json` (`version/platform/publishedAt/changelog/fileName/size/sha256/downloadUrls`, where the URLs are the Connect fixed-tag `windows` link first and the GitHub direct link second) and signs it with `go run ./tools/updatesign sign`, which produces an Ed25519 envelope `{manifest: base64(raw bytes), signature}`. It re-verifies the envelope with the client's embedded public key (`TestReleaseEnvelope`), commits it as `latest.json` to the public repo's `main` via the contents API, and purges jsDelivr. Secrets: `RELEASES_REPO_TOKEN` (fine-grained PAT, Contents read/write on the public repo only; when absent the whole block is skipped) and `UPDATE_SIGNING_KEY` (base64 32-byte seed from `go run ./tools/updatesign keygen`). OpenList (`drive.czl.net`) is intentionally not used: its GitHub Releases driver cannot proxy private-repo assets even with web proxy on (it fetches without the token and gets 404). `downloadUrls` is a list, so mirrors can change without touching the client. The landing page at www (`czl-browser/BrowserClient.tsx`) links to `connect.czl.net/dl/czlbrowser/latest/windows`.

**Kernel list pipeline** (lives in the public `woodchen-ink/Chrome-exe`; czlbrowser is private, so users can't download its release assets). `sync-chrome-stable.yml` runs daily at UTC 01:00 and has two jobs:
- `sync` queries Google's Omaha update server (`POST tools.google.com/service/update2`, Chrome appid, `ap=x64-stable-multi-chrome`) for version/URLs/sha256/size (it used `Bush2021/chrome_installer`'s `data.json` until that account vanished in 2026-09), downloads the official offline installer from Google domains only, verifies Google's SHA256 and size, extracts `Chrome-bin/` with 7z, and publishes `Chrome-bin.zip` + `.sha256` as a Release tagged with the version. 7z quirks: newer 7-Zip extracts `Chrome-bin/` directly while older p7zip needs a second pass on `chrome.7z`; exit code 1 is only a warning.
- `manifest` (`if: always()`) updates the repo's `cores.json`, which the client fetches at runtime, then commits and purges jsDelivr. The entries update independently: one failing upstream still lets the others update, and the job fails at the end.
  - `chrome-stable` = this repo's newest `Chrome-bin.zip` release (tag = version).
  - `clearcote` = the newest non-draft release of `clearcotelabs/clearcote-browser` that has `clearcote-<ver>-windows-x64.zip`. Upstream `releases/latest` is the SDK (`sdk-v*`) and every browser build is a pre-release, so never use `latest`.

Every entry requires an `<asset>.sha256`. Regexes use `[.]` instead of `\.` so they survive shell escaping on every platform. The whole pipeline was verified end to end with a real run: `cores.json` was seeded with the stale clearcote pre.21, and the job moved it to pre.22.

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
  app_instance_kernel.go         # Per-kernel launch plan (arg translation, CDP vs native switches)
  app_instance_stealth.go        # CDP stealth/timezone injection (standard Chrome only)
  app_builtin_cores.go           # Built-in kernel list API
  app_builtin_cores_manifest.go  # Remote kernel manifest fetch, cache, embedded fallback
  app_update.go                  # App self-update API (UpdateCheck/UpdateDownload/UpdateInstall), AppVersion
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
      core_kind.go               # Kernel type detection (chrome / clearcote)
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
    backup/                      # Backup spec and format
    tray/                        # System tray integration
    update/                      # Signed update manifest (Ed25519), version compare, verified download, installer launch
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

- **Install root & layout** (critical, follows the CZL desktop install rule): the installer does a per-user install into the fixed root `%LOCALAPPDATA%\CZL\CZL Browser` (`config.InstallRoot()`). It uses `RequestExecutionLevel user`, has no directory page, writes the uninstall key to HKCU and never triggers UAC. That root holds `czl-browser.exe`, `bin/`, `config/config.yaml`, `data/`, `chrome/` and `logs/`. Roaming `%APPDATA%` is never used. If `LOCALAPPDATA` is missing, startup exits with an error and there is no fallback to the cwd or exe dir. Upgrades replace only program files. The installer writes its default config only when neither the new config nor a legacy one exists. Uninstall always removes the exe, `bin/` and shortcuts, and asks before deleting `config/data/chrome/logs` (the default keeps them). **Legacy layout migration**: older builds installed into `C:\Program Files\CZL Browser` (HKLM key) with config at `%APPDATA%\CZL Browser\config.yaml` and data at `%LOCALAPPDATA%\CZL Browser`. The installer's `.onInit` detects the HKLM key and offers to run the old uninstaller through `ExecShellWait` so UAC can elevate it. On production startup, `config.MigrateLegacyLayout` renames legacy entries into the new root before loading config. Directories are merged entry by entry, existing target files are never overwritten, failures are logged and don't block startup, and emptied legacy dirs are removed. Stored paths are relative to `dataRoot`, so they survive the move.
- **Paths & data root**: `appRoot` = exe directory (dev: project root) — bundled resources only (`bin/xray.exe`, `bin/sing-box.exe`). `dataRoot` = writable data root: dev = project root, production = the install root. `configPath`: dev = `<project>/config.yaml`, production = `<install root>/config/config.yaml` (`config.ConfigPathIn`). `main.go` resolves both and injects them via `App.SetLayout(dev, dataRoot, configPath)`. All writable data — `data/` (db, snapshots, auth, user-data), `chrome/` kernels, logs, proxy workdirs (`data/_xray`, `data/_singbox`) — resolves under `dataRoot` via `App.resolveAppPath` / `browser.Manager.ResolveRelativePath`. `config.yaml` reads/writes go through `App.mainConfigPath()` / `browser.Manager.configSavePath()` (→ `configPath`, falling back to `dataRoot/config.yaml` when unset, e.g. tests), never via `resolveAppPath`. Never put `--user-data-dir` under `C:\Program Files`: Chromium's mojo IPC and network sandbox fail with `拒绝访问 (0x5)` there, and the process exits before its debug port opens. `killResidualRuntimeProcesses(appRoot, dataRoot)` scans both roots (bin under appRoot, chrome under dataRoot).
- **Database**: SQLite (`data/app.db` under `dataRoot`) via DAO pattern. Auto-migrated on startup.
- **Config**: `config.yaml` at `configPath` (production `<install root>\config\config.yaml`). Loaded with auto-repair on missing fields. Supports runtime reload via `ReloadConfig()`.
- **Proxy runtime**: `bin/xray.exe` and `bin/sing-box.exe` shipped in-repo, resolved under `appRoot` (exe dir). No external downloads needed.
- **Chrome kernels**: Placed in `chrome/` directory under `dataRoot`, either user-provided or downloaded from the built-in list. Managed through the UI's core management page. The built-in list is fetched at runtime by `app_builtin_cores_manifest.go`: raw.githubusercontent.com and jsDelivr copies of `Chrome-exe/main/cores.json` are requested concurrently with a 6s timeout and the system proxy, and the first valid response wins. A good result is cached to `data/builtin_cores_cache.json` and reused in memory for 10 minutes; `BrowserBuiltinCoresRefresh` forces a refresh. On failure it falls back to that cache, then to the embedded `backend/builtin_cores.json`. Entry fields: `id/name/version/kind/release/description/downloadUrl/sha256`. `parseBuiltinCoreManifest` drops any entry whose download URL isn't `https://github.com/…`, whose sha256 isn't 64 hex characters, or whose id is a duplicate. When `sha256` is non-empty, `DownloadAndExtractCore` verifies the zip before extracting it. Don't rename the ids `clearcote` / `chrome-stable`, because the pipeline keys on them. The core management page loads this list without blocking the local core table.
- **Switching an instance's kernel**: verified on one profile, going Chrome 153 → Clearcote 149 → 153 → 149. Cookies (including HttpOnly), localStorage and sessions survived each step, and no downgrade snapshot or wipe happened. That works because portable kernels encrypt with DPAPI only (`os_crypt.encrypted_key`, no `app_bound_encrypted_key`), and all 28 real local instances were checked to be the same. The kind is re-detected at every launch, so args always match the current kernel. What does change is the fingerprint seen by sites (brand/GPU/canvas implementation), and large version downgrades can hit DB schema incompatibilities. `BrowserEditPage` warns in both cases, when the kind differs or the target major version is lower. `GetChromeVersion` also reads the `<version>/` dir name for the official `Chrome-bin` layout, which has no root `.manifest`.
- **Kernel kinds & launch strategy**: `browser.DetectCoreKind(chromeExe)` looks for Clearcote's own switch string `canvas-bridge-url` inside `chrome.dll` (at the kernel root for Clearcote, under `<version>/` for official Chrome) and caches the result in memory keyed by dll path + mtime + size. No DB column is involved, so manually added and backup-imported kernels are covered too. `buildKernelLaunchPlan` then branches on the kind. **chrome** (official Google Chrome, which ignores every `--fingerprint-*`/`--timezone` switch; the `TZ` env var was verified to have no effect as well): keep the short-format `--fingerprint-webgl-*` args and inject stealth + timezone over CDP after launch. **clearcote** (engine-level fingerprinting, BSD-3, verified on 149): no CDP injection at all. Args are translated: `--fingerprint-webgl-vendor/renderer` → `--fingerprint-gpu-vendor/renderer` in the full ANGLE format real Chrome reports (Clearcote returns values verbatim, and short strings are a detection tell), `--fingerprint-touch-points` → `--fingerprint-max-touch-points`, platform `mac` → `macos`, and the effective timezone is passed as the native `--timezone`, which reaches every renderer so pages, cross-site iframes, workers and popups all agree (the upstream PATCHES.md mentions `--clearcote-timezone`, but that name is stale and has no effect). Without `--fingerprint-brand` Clearcote reports the Chromium brand.
- **Singleton cleanup**: before launching an instance, `cleanupChromeSingletonLocks` removes stale `SingletonLock`/`SingletonCookie`/`SingletonSocket` from the profile dir — these point at the old host after a cross-machine backup import and otherwise make Chromium exit immediately. Runs alongside `patchChromePreferencesForCleanStart`.
- **Backup export/import**: Full backup (`BackupExportPackage`/`BackupImportPackage`) packages config, SQLite, proxies and browser user-data into a ZIP. Two exclusions keep the package small and config-focused: (1) Inside each Chromium profile only real account data is kept; everything rebuildable is dropped. Profile roots are detected structurally via `backup.DetectProfileRoots` (a dir directly containing `Default/` or `Local State` is a profile root — works for UUID and user-named `UserDataDir` alike, not by name matching). `backup.NewProfileAwareSkip` then whitelists `Default`/`Profile N`/`System Profile` data dirs plus key top-level files (`Local State` etc.), dropping component-updater dirs (`WasmTtsEngine`/`WidevineCdm`/`OnDeviceHeadSuggestModel`/…), telemetry (`BrowserMetrics`/`DeferredBrowserMetrics`), in-`Default` rebuildable caches (`Cache`/`Code Cache`/`GPUCache`/`Service Worker` cache), and blocked extension bodies under `Extensions/` for Adobe Acrobat (`efaidnbmnnnibpcajpcglclefindmkaj`), Google Docs Offline (`ghbmnnjooekpmoecnnnilnnbdlolhkhi`), IDM Integration Module (`ngpampappnmepgilojfohadhhmbhlaek`), and Chrome Web Store Payments (`nmmhkkegccagdldgiimedpiccmgmieda`). Bitwarden (`nngceckbapebfimnlniiiahkandclblb`) and `Local Extension Settings` are kept. The same blocked extension bodies are removed again before launching a profile so historical imports are self-healed. Non-profile data under `data/` (SQLite, snapshots, auth) is always kept. Applied on both export (per scope entry) and import (`backupImportFileTrees`). This drops a real ~3GB `data/` to ~250MB. (2) Kernel binaries (`chrome/` under `dataRoot` and config-referenced external cores) are not exported — only their path references travel with `config.yaml`/`browser_cores`; the user re-downloads/re-places same-named kernels on a new machine (into `%LOCALAPPDATA%\CZL\CZL Browser\chrome` in production). Import still restores kernels from older packages that contain them. Backup scope is built from `dataRoot` (data/db/logs/chrome) with `config.yaml` sourced separately from `configPath`.
- **Timezone spoofing**: Browser launch strips the stored `--timezone` flag and resolves the effective timezone (proxy exit locale, falling back to the manual value). Clearcote gets it back as a native switch (see above). Standard Chrome gets a timezone override through browser-level CDP auto-attach so existing and newly opened page targets stay aligned with the proxy exit locale (or manual timezone fallback). Every attached session (any target type, injected or not) must get `Runtime.runIfWaitingForDebugger` (`resumeTargetIfWaiting`): popups from `window.open` are paused on attach even with `waitForDebuggerOnStart:false`, and since they share the opener's renderer, skipping the resume leaves the new tab at `about:blank` and freezes the opener under a "已在调试程序中暂停" overlay. `stealthScript` is wrapped in an IIFE because it runs twice per document (`addScriptToEvaluateOnNewDocument` + `Runtime.evaluate`), and top-level `const` redeclaration would throw (surfacing as a `window.open` exception in popups).
- **Auth**: OAuth2 via CZL Connect (`connect.czl.net`). Encrypted token + user info persisted to `data/auth_token.json` (`storedSession` envelope; legacy token-only files auto-migrated). On startup `TryRestoreSession` restores synchronously from disk (no network) so login survives restart and tolerates offline/weak network; `RefreshSessionAsync` then refreshes the token and re-fetches user info in the background, emitting `auth:restored` / `auth:session:expired` to the frontend. A token is only cleared on definitive refresh failure (`invalid_grant`), not on transient userinfo errors. App requires login to use — OAuth2 provides identity only. CZL Connect also sends a `groups` permission array in `userinfo`, but the app is free/open-source and does not tier users: `auth.UserInfo` deliberately omits the field (`encoding/json` drops it), `AuthState` carries no `level`, and no user-level/permission-group UI exists. Re-introducing tiers means re-adding the field plus its parsing — the payload shape has changed before (comma string → array), so parse defensively. Auth state managed by `authStore` (Zustand).
- **App self-update**: `update.FetchLatest` requests the same signed `czlbrowser-releases/main/latest.json` concurrently from CZL Connect (`/dl/czlbrowser/latest/update-manifest`, primary, about 60s cache), raw.githubusercontent and jsDelivr (8s, system proxy). It waits for every response and keeps the **highest** version that passes verification, so a stale cached copy can't hide a new release. `ParseEnvelope` verifies the signature against `update/pubkey.go` before trusting anything. It also requires `platform=windows-x64`, a `fileName` matching `CZLBrowser-Setup-<digits/dots>.exe` (the name is used as a local path), a size of at most 512MB, a 64-hex sha256 and https URLs. Rotating the key means changing `pubkey.go` and shipping a build signed with the old key first. `UpdateDownload` downloads into `data/_update/` in the background and tries the URLs in order. A 30s stall watchdog moves on to the next URL. The size and sha256 are checked on the stream, and the `.part` file is renamed only when they match. Progress is sent as `update:progress` (`downloading/ready/error`), and older installers are removed. `UpdateInstall` verifies the file again and starts `installer /S /RELAUNCH /WAITPID=<pid>` detached, then calls `ForceQuit`, which stops instances and proxies gracefully. NSIS `.onInit` waits up to 60s for that PID (`Wait-Process`) before its usual process cleanup, and `.onInstSuccess` relaunches the app when `/RELAUNCH` is present. In silent mode MessageBoxes use `/SD` defaults. Frontend: `AuthenticatedApp` runs `useAppUpdate` (check at login, then every 6h, one notification per version), `store/updateStore.ts` holds the state, the Sidebar shows a dot on 系统设置 when an update exists, and `modules/settings/UpdateCard.tsx` is the 软件更新 card (check, download, confirm install and restart).
- **Profile quota**: Instance create/copy flows are not blocked by `MaxProfileLimit`; quota values are informational only.
- **Quota UI**: Dashboard and Browser List no longer expose redeem/expand quota entry points.

## Key Conventions

- **Language**: All UI text, log messages, and comments are in Chinese
- **Go module**: `czl-browser` (not `czl-browser`)
- **CSS**: Tailwind CSS with CSS variables defined in the theme (no hardcoded Tailwind colors). CZL neutral palette: black/white/gray dominant; `--color-accent` is **near-black** (drives links/selection/active/icon coloring), brand blue `--color-brand` (`#2EA7E0`) only appears in the focus ring (`--color-ring`) and the Logo — never as link/button/selection color. No gradients in UI. Charts use `--color-chart-1`..`6`.
- **State management**: Zustand (lightweight stores in `src/store/`)
- **Icons**: Lucide React only (no emoji in UI)
- **Frontend routing**: React Router v6, routes defined in `App.tsx`
- **Backend method naming**: `BrowserProfile*`, `BrowserProxy*`, `BrowserCore*` prefixes for Wails-bound methods
- **Type aliases**: `backend/app.go` re-exports `browser.*` types (e.g., `type BrowserProfile = browser.Profile`) for Wails binding compatibility

## Branches

- `master`: Clean development baseline, no user data
- `user_data`: Includes test `data/app.db` for demo/debugging

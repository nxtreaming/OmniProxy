# AI Constraints for OmniProxy

This document is for AI coding agents and automation tools working in this repository. Follow it before making code, build, release, or documentation changes.

## Project Identity

- OmniProxy is a local desktop application, not a hosted web app.
- The backend project lives in `OmniProxyBackend/`; the Vue frontend lives in root `frontend/`.
- The desktop shell is Wails v2. The backend is Go. The UI is Vue 3 + Vite + Element Plus.
- The frontend is built from `frontend/` and copied into `OmniProxyBackend/frontend-dist/` for Go embed.
- Default local ports are:
  - Proxy: `127.0.0.1:3000`
  - Control API: `127.0.0.1:3890/api`

## Hard Rules

- Do not treat this as a standalone web deployment unless the user explicitly asks for web-only work.
- Do not expose the proxy or control API beyond `127.0.0.1`.
- Do not commit secrets, tokens, `auth.json`, `.env`, `tokens.json`, exported credential backups, or local data directories.
- Do not print real token values, Codex auth JSON, control tokens, or DPAPI-protected credential material in logs, tests, docs, or final answers.
- Do not remove or weaken control-token protection for HTTP control API endpoints.
- Do not change data directory behavior without considering migration, bootstrap pointer files, and `OMNIPROXY_DATA_DIR`.
- Do not revert unrelated user changes in the working tree.
- Do not run destructive Git or filesystem commands unless the user explicitly asks for them.

## Build Expectations

Use Wails to build the desktop application.

Normal desktop build:

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

Dev-version exe build without rebuilding frontend:

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build -s -clean -nopackage -tags omniproxy_dev -o OmniProxy-dev.exe -ldflags "-X main.appVersion=dev"
```

Notes:

- `-s` skips the frontend build and embeds the existing `OmniProxyBackend/frontend-dist`.
- `-tags omniproxy_dev` marks the executable as the dev instance, with a separate single-instance lock, data directory, bootstrap file, autostart key, and default ports.
- If the UI source changed and the user wants those changes inside the exe, build the frontend first or run Wails without `-s`.
- Do not run `npm run build` when the user explicitly asks not to build the web/frontend side.
- Build output goes to `OmniProxyBackend/build/bin/`.
- After a successful dev-version exe build, automatically start `OmniProxyBackend/build/bin/OmniProxy-dev.exe` unless the user explicitly asks not to launch it. Do not stop or replace a running production `OmniProxy.exe`.

## Dev and Production Coexistence

Development executables must be able to run at the same time as production executables.

When building `OmniProxy-dev.exe`, always use the dev build tag:

```powershell
-tags omniproxy_dev
```

Do not build the dev executable with only `-ldflags "-X main.appVersion=dev"`. The version flag changes the displayed version, but the build tag is what selects the compile-time dev instance mode.

Expected runtime separation:

- Production single-instance ID: `com.omniproxy.desktop.production`
- Dev single-instance ID: `com.omniproxy.desktop.dev`
- Production default data dir: `%USERPROFILE%\.omniproxy`
- Dev default data dir: `%USERPROFILE%\.omniproxy-dev`
- Production bootstrap file: `%APPDATA%\OmniProxy\bootstrap.json`
- Dev bootstrap file: `%APPDATA%\OmniProxyDev\bootstrap.json`
- Production default proxy/control ports: `3000` / `3890`
- Dev default proxy/control ports: `3001` / `3891`
- Production autostart key: `OmniProxy`
- Dev autostart key: `OmniProxy Dev`

If code changes touch instance mode, config defaults, data directory resolution, startup behavior, tray behavior, or autostart behavior, verify both profiles:

```powershell
cd .\OmniProxyBackend
go test ./...
go test -tags omniproxy_dev ./...
```

For dev desktop builds, use this full command unless the user explicitly asks for a different build:

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build -s -nopackage -tags omniproxy_dev -o OmniProxy-dev.exe -ldflags "-X main.appVersion=dev"
```

After the command succeeds, launch the dev executable:

```powershell
Start-Process -FilePath .\build\bin\OmniProxy-dev.exe -WorkingDirectory .\build\bin
```

## Verification Commands

Backend tests:

```powershell
cd .\OmniProxyBackend
go test ./...
```

Frontend tests:

```powershell
cd .\frontend
cmd /c npm test
```

Frontend production build, only when needed:

```powershell
cd .\frontend
cmd /c npm run build
```

Use the smallest verification set that matches the change:

- Go-only backend changes: run `go test ./...`.
- Frontend logic or formatting utilities: run `cmd /c npm test`.
- UI/template/CSS changes that must be embedded: run frontend build or Wails build as requested.
- Wails exported method changes: run Go tests and confirm frontend bindings are updated.

## Go Backend Constraints

- Keep provider routing, scheduling, retry, validation, and usage accounting inside `internal/proxy` and `internal/token` boundaries unless a broader refactor is requested.
- Use typed structs and existing normalization helpers instead of ad hoc map or string handling.
- Keep persistence local and explicit:
  - Config: `internal/config`
  - Token state: `internal/token`
  - Request history: `internal/history`
  - Logs: `internal/logs`
- Any exported `DesktopApp` method used by the UI must remain compatible with Wails bindings.
- If adding or changing a Wails method, update generated-style files under `frontend/wailsjs/go/` when Wails generation changes them.
- Preserve local-only networking defaults. Listeners should bind to loopback unless the user asks for a different security model.
- Tests should use `httptest`, temp directories, and local fake upstreams. Do not call real provider APIs in tests.

## Frontend Constraints

- Use Vue 3 composition API patterns already present in `frontend/src/App.vue`, `frontend/src/components/`, and `frontend/src/features/`.
- Use `frontend/src/services/api.js` for backend access. It must prefer Wails bindings and fall back to the local HTTP control API.
- Do not fetch backend endpoints directly from components unless there is a clear local precedent.
- Keep Element Plus as the UI component system.
- Keep navigation tabs in `frontend/src/constants/app.js`.
- Keep shared formatting in `frontend/src/utils/format.js`.
- Keep page-specific views under `frontend/src/features/`; keep reusable widgets in `frontend/src/components/`.
- Avoid adding large global state libraries unless the user asks for a larger frontend architecture change.
- Design should remain a dense desktop control console, not a marketing landing page.

## Update and Version Constraints

- Runtime app version is controlled by `main.appVersion`.
- Release update checks use GitHub latest release metadata in `update.go`.
- Development builds use version `dev` and should not call the release API during update checks.
- To set a release/dev version at build time, pass Go ldflags:

```powershell
-ldflags "-X main.appVersion=<version>"
```

- Do not hardcode a release version in source unless the release process explicitly requires it.

## Release Constraints

When the user asks to publish a release, use GitHub Actions. Do not build release artifacts locally and push/upload them manually.

Required release flow:

- Use `.github/workflows/release.yml` as the release path.
- The release workflow is triggered by a `v*.*.*` tag push or by `workflow_dispatch` with a `tag` input.
- Prefer GitHub Actions `workflow_dispatch` when the user asks to publish an existing tag.
- Local Wails builds are allowed only for verification or dev handoff. They are not release artifacts.
- Do not upload local `build/bin/*.exe`, local NSIS installers, or local checksum files to GitHub Releases.
- If a release note file is needed, create or update `docs/releases/vX.Y.Z.md`; the workflow reads that curated note file when present.
- The release version must use a tag like `v1.2.3`.
- Before release, verify the intended tag, compare range, and release notes with the user if they are ambiguous.

Release notes must use this bilingual template:

```markdown
# [Version / 版本号] - [Highlight / 核心亮点]

[English short introduction of the core updates in this release.]
[简短的中文引言，概括这个版本最重要的变化。]

##  Features / 新特性
- **[Module]**: [English description of the feature] (#PR) @Contributor
  - [中文功能描述]
- **[Module]**: [English description of the feature] (#PR) @Contributor
  - [中文功能描述]

##  Bug Fixes / 问题修复
- **[Module]**: Fixed [English description of the bug] (#PR) @Contributor
  - 修复了 [中文缺陷描述]

## ⚠️ Breaking Changes / 不兼容更新
- **[Attention]**: [English description of the breaking change] (#PR)
  - **[必读]**: [中文破坏性变更描述及迁移指南]

##  Refactor & Performance / 重构与优化
- **[Module]**: Improved [English description] (#PR)
  - 优化了 [中文优化描述]

---
**Full Changelog / 完整更新日志**: https://github.com/你的用户名/你的项目/compare/v上一个版本...v当前版本
```

When filling the template:

- Replace placeholder modules, PR numbers, contributors, versions, and URLs with real values.
- Use the real repository compare URL, currently `https://github.com/mibgb65-cloud/OmniProxy/compare/vPREVIOUS...vCURRENT`.
- Omit empty sections only if the user explicitly allows it; otherwise keep the section and write `- None / 无`.
- Keep English and Chinese entries paired.
- Keep release notes concise and user-facing. Do not paste raw commit logs unless there are no curated notes and the user accepts generated notes.

## Security and Privacy Constraints

- Real credentials must never be shown in UI, logs, tests, docs, or examples.
- Mask credentials consistently. For Codex auth JSON, display only safe labels such as `auth.json`.
- Exported token backups and Codex auth files contain secrets. Treat them as sensitive.
- The control API token endpoint must use no-store cache behavior.
- Control API calls outside Wails must include `X-OmniProxy-Control-Token` or bearer auth, except `GET /api/control-token`.
- When adding diagnostics, prefer status, provider, route, duration, and masked account labels over raw request headers or bodies.

## Documentation Constraints

- Keep Chinese README and English README consistent when changing user-facing documented behavior.
- Use Windows PowerShell commands for primary examples because this project is currently Windows/Wails focused.
- Avoid documenting fake provider URLs or credentials that could be mistaken for real secrets.
- When documenting build outputs, use paths under `OmniProxyBackend/build/bin/`.

## Working Tree Constraints

- Check `git status --short` before and after substantial changes.
- Existing dirty files may belong to the user. Work around them rather than reverting them.
- Build artifacts in ignored directories should not be staged unless the user explicitly asks for binary deliverables to be committed.
- Wails build may update generated binding files. Review those diffs before finalizing.

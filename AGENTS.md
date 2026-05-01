# Codex Project Guide

This file is for Codex and other coding agents working in this repository.

## Project Overview

OmniProxy is a local desktop gateway for AI API clients. It manages multiple provider credentials, routes requests through a local proxy, records usage, refreshes quota state, and can configure clients such as Codex and Claude Code to use the local proxy.

Main stack:

- Backend and desktop shell: Go + Wails v2 in `OmniProxyBackend/`.
- Frontend: Vue 3 + Vite + Element Plus in `OmniProxyBackend/frontend/`.
- Local data: `%USERPROFILE%\.omniproxy` by default.
- Main README: `README.md`.

## Repository Layout

- `OmniProxyBackend/`: Wails app, Go backend, generated bindings, resources.
- `OmniProxyBackend/internal/config/`: local config, defaults, data directory handling.
- `OmniProxyBackend/internal/proxy/`: HTTP proxy, provider routing, auth injection, retry behavior, WebSocket proxying.
- `OmniProxyBackend/internal/token/`: account models, token pool, scheduling, quota and usage state.
- `OmniProxyBackend/internal/storage/`: local JSON persistence.
- `OmniProxyBackend/internal/logs/`: in-memory request and diagnostic logs.
- `OmniProxyBackend/frontend/`: Vue frontend.
- `scripts/dev.ps1`: Wails desktop development launcher.

## Working Rules

- Keep changes scoped to the user request. Avoid broad refactors unless they are needed to finish the task safely.
- Check the current worktree before editing. Preserve user changes and do not revert unrelated modifications.
- Treat credentials as sensitive. Do not commit real `auth.json`, `.env`, `tokens.json`, exported account backups, access tokens, API keys, or local logs containing secrets.
- Prefer existing package boundaries and naming. Backend business logic usually belongs under `internal/`; UI-only behavior belongs under `frontend/src/`.
- Use Chinese for user-facing copy when editing existing Chinese UI or docs, unless the surrounding file is English.
- When changing Wails-exposed Go methods or response models, keep the generated `frontend/wailsjs/` bindings in sync.
- Do not expose the control API or proxy listener beyond loopback unless the user explicitly requests that behavior.

## Common Commands

Run desktop app in development:

```powershell
.\scripts\dev.ps1
```

Run backend tests:

```powershell
cd .\OmniProxyBackend
go test ./...
```

Run frontend tests:

```powershell
cd .\OmniProxyBackend\frontend
npm test
```

Build frontend:

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

Build desktop app:

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

## Validation Guidance

- For backend routing, scheduling, quota refresh, storage, or model changes, run `go test ./...` from `OmniProxyBackend/`.
- For frontend service utilities or formatting helpers, run `npm test` from `OmniProxyBackend/frontend/`.
- For frontend component or CSS changes, run `npm run build` when feasible.
- For Wails binding changes, run a desktop build or development launch when feasible.
- If a command cannot be run in the current environment, report that clearly with the reason.

## Security Notes

- The app is designed for local personal development and should bind to `127.0.0.1` by default.
- Windows credential persistence relies on the current user's DPAPI context. Be careful when changing token storage, import, export, or migration logic.
- Exported account pools and Codex `auth.json` files contain real credentials. Keep generated examples fake and obvious.
- Logs and screenshots can contain account names, paths, request IDs, or provider-specific metadata. Sanitize before adding samples to docs or tests.

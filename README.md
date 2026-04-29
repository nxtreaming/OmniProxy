# 🚀 OmniProxy

> 本地 AI API 令牌调度、额度观测与无感代理网关。一个桌面应用，把多账号、多厂商、多客户端工具收拢到同一个本地控制台里。

[English](README_EN.md) · 中文

OmniProxy 面向本地 AI 开发工作流设计。它可以让 Codex、Claude Code 以及兼容 OpenAI / Anthropic 协议的客户端先连接到本机代理，再由 OmniProxy 根据账号状态自动选择可用账号、记录用量、刷新额度、失败重试并切换账号。

如果你经常遇到这些场景，OmniProxy 大概率能帮上忙：

- 🪫 单个账号额度耗尽，客户端请求直接失败。
- 🔁 多个账号来回手动切换，配置文件越改越乱。
- 🧩 OpenAI、Anthropic、DeepSeek、Kimi、Xiaomi MiMo 的入口和鉴权方式不一致。
- 🕵️ 请求到底用了哪个账号、用了多少 Token、失败在哪里都不透明。
- 🛠️ Codex / Claude Code 本地配置想自动写入，也希望能一键恢复。

## ✨ 核心亮点

- 🖥️ **桌面端闭环**：Wails + Go + Vue 3，一站式管理账号、代理、日志、额度和设置。
- 🔐 **本地透明代理**：客户端只连 `127.0.0.1`，真实上游 Token 由 OmniProxy 自动注入。
- 🧠 **多账号调度**：支持队列模式和优先平衡使用模式，并避开正在请求中的账号。
- 🧯 **失败自动切换**：遇到 `429`、`502`、`503`、`504` 等错误时可换账号重试。
- 📊 **额度与用量观测**：展示剩余额度、重置时间、请求次数、输入 / 输出 / 总 Token。
- 📈 **历史统计分析**：按日期、厂商、模型和失败原因汇总请求历史，并展示模型 Token 饼图。
- ⚡ **当前账号额度刷新**：正在使用的 Codex 和可验证账号会每 30 秒自动刷新额度状态。
- 🧭 **Claude Code 快速接入**：支持将 Claude Code 指向 DeepSeek、Kimi、Xiaomi MiMo 的本地路由。
- 🧵 **Codex WebSocket 代理**：可在设置页开启或关闭，并继续记录请求用量。
- 🧱 **本地持久化**：配置、账号、统计数据写入本地文件，Windows 上账号凭据使用 DPAPI 加密落盘。
- 📤 **凭据导出**：支持导出完整账号池备份，也可以把 Codex auth.json 按账号导出为独立文件。
- 🎨 **更顺手的控制台**：页面切换动画、当前账号高亮、导航图标、桌面图标已统一。

## 🧠 工作方式

```text
Codex / Claude Code / API Client
              |
              v
     http://127.0.0.1:3000
              |
              v
        OmniProxy Gateway
   Token Pool + Scheduler + Logs
              |
              v
OpenAI / Anthropic / DeepSeek / Kimi / Xiaomi MiMo
```

OmniProxy 不改变客户端的使用方式。你只需要把客户端的 Base URL 指向本机代理，后续账号选择、鉴权头注入、重试切换、额度更新和日志记录都由本地程序完成。

## 📦 项目结构

```text
.
├── OmniProxyBackend/              # Wails 桌面主工程与 Go 后端
│   ├── internal/config/           # 本地配置、数据目录、默认值
│   ├── internal/logs/             # 内存日志记录器
│   ├── internal/proxy/            # 代理、路由、鉴权、用量解析、WebSocket
│   ├── internal/storage/          # JSON 本地存储
│   ├── internal/token/            # Token 池、调度、状态、统计
│   └── frontend/                  # Vue 3 + Vite + Element Plus 前端
├── scripts/dev.ps1                # Wails 桌面开发启动脚本
├── README.md                      # 中文文档
└── README_EN.md                   # English README
```

默认端口和数据位置：

| 项目 | 默认值 |
| --- | --- |
| 🧩 控制 API | `http://127.0.0.1:3890/api` |
| 🚪 代理服务 | `http://127.0.0.1:3000` |
| 💾 本地数据 | `%USERPROFILE%\.omniproxy` |

## ⚡ 快速开始

### 1. 准备环境

- Go
- Node.js
- Wails v2 CLI

### 2. 开发运行

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe dev
```

也可以使用脚本：

```powershell
.\scripts\dev.ps1
```

### 3. 构建桌面应用

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

构建产物：

```powershell
.\OmniProxyBackend\build\bin\OmniProxy.exe
```

## 🧭 使用流程

1. 🚀 启动 OmniProxy。
2. 🔑 在「账号管理」里添加 OpenAI、Anthropic、DeepSeek、Kimi 或 Xiaomi MiMo 账号。
3. ⚙️ 在「全局设置」里确认代理端口和各厂商 Base URL。
4. 🟢 启动本地代理。
5. 🧩 将 Codex、Claude Code 或其他 API 客户端指向本地代理地址。
6. 📊 在「仪表盘」和「额度」页面查看当前账号、额度重置时间、Token 明细和实时日志。

常见代理地址：

```text
OpenAI compatible: http://127.0.0.1:3000
Codex backend:     http://127.0.0.1:3000/backend-api/codex
Claude router:     http://127.0.0.1:3000/anthropic-router
```

桌面端也提供「一键配置」入口，可将本机 Codex 或 Claude Code 写入 OmniProxy 本地代理地址，并保留原始配置备份用于恢复。

## 🔌 支持的账号类型

| 厂商 | 凭据类型 | 说明 |
| --- | --- | --- |
| OpenAI | API Key | 使用 `Authorization: Bearer` |
| OpenAI / Codex | `auth.json` | 自动解析邮箱、access token、account id，并刷新 Codex 额度 |
| Anthropic | API Key | 使用 `x-api-key` |
| DeepSeek | API Key | 支持 OpenAI 兼容和 Anthropic 路由 |
| Kimi | API Key | 支持 Kimi Code 相关路由 |
| Xiaomi MiMo | API Key | 按量 API Key，通常以 `sk-` 开头 |
| Xiaomi MiMo | Token Plan | Token Plan Key，通常以 `tp-` 开头 |

## 🧰 控制 API

桌面前端优先通过 Wails 绑定调用后端。控制 API 仍保留给本地调试、外部脚本和兼容旧开发方式使用：

- `GET /api/control-token`
- `GET /api/tokens`
- `POST /api/tokens`
- `PUT /api/tokens/{id}`
- `DELETE /api/tokens/{id}`
- `POST /api/tokens/{id}/validate`
- `GET /api/config`
- `PUT /api/config`
- `GET /api/logs`
- `GET /api/proxy/status`
- `POST /api/proxy/start`
- `POST /api/proxy/stop`
- `POST /api/codex/configure`
- `POST /api/codex/restore`
- `POST /api/mimo/claude/configure`
- `POST /api/mimo/claude/restore`
- `POST /api/deepseek/claude/configure`
- `POST /api/deepseek/claude/restore`
- `POST /api/kimi/claude/configure`
- `POST /api/kimi/claude/restore`

使用 HTTP 控制 API 时，除 `GET /api/control-token` 外，其它接口需要带上 `X-OmniProxy-Control-Token` 请求头；也可以使用 `Authorization: Bearer <token>`。

## ✅ 验证命令

后端测试：

```powershell
cd .\OmniProxyBackend
go test ./...
```

前端测试：

```powershell
cd .\OmniProxyBackend\frontend
npm test
```

前端构建：

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

桌面打包：

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

## 🛡️ 安全说明

OmniProxy 面向本地个人开发场景设计，默认只绑定 `127.0.0.1`。账号凭据保存在你选择的数据目录中，不会主动上传到任何第三方服务。Windows 上 `tokens.json` 中的真实凭据会使用当前用户的 DPAPI 加密，迁移到其它机器或用户前请先使用桌面端导出功能备份。

仍然建议：

- 🚫 不要把 `auth.json`、`.env`、`tokens.json` 或任何真实凭据提交到 Git。
- 🔒 不要把控制 API 暴露到公网或局域网。
- 🧪 使用前确认本地代理端口没有被其他程序占用。
- 📤 导出的账号池备份和 Codex auth.json 文件包含真实凭据，请只保存到可信目录。
- 🧹 分享日志或截图前检查是否包含账号名、路径或敏感配置。

## 🗺️ 路线图

- 📈 增加额度趋势图、请求历史图表和更细粒度的统计视图。
- 🔐 继续收紧控制 API 的安全边界，补充更细的本地访问控制。
- 🧩 完善更多厂商和更多协议适配。
- 🧪 增加 SSE、WebSocket、并发调度和异常恢复的端到端测试。
- 🧱 继续拆分前端状态和页面结构，让组件边界更清晰。
- 📦 优化发布流程，降低桌面端打包和分发成本。

## 🤝 参与贡献

欢迎一起把 OmniProxy 打磨成更顺手的本地 AI 网关。

如果你遇到了问题，欢迎提交 Issue，并尽量附上：

- 🖥️ 操作系统和运行方式。
- 🧰 使用的客户端工具，例如 Codex、Claude Code 或自定义 API 客户端。
- 🔌 相关厂商、路由路径和错误日志。
- 🎯 预期行为和实际行为。

如果你想贡献代码，欢迎提交 Pull Request。比较适合优先参与的方向：

- ✨ 新厂商或新协议适配。
- 🧠 代理稳定性、并发调度、重试策略优化。
- 🎨 前端交互、可视化、配置体验改进。
- ✅ 测试覆盖、文档示例、问题复现用例。

提交 PR 前建议先跑测试和构建：

```powershell
cd .\OmniProxyBackend
go test ./...
```

```powershell
cd .\OmniProxyBackend\frontend
npm test
```

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

## ⭐ 支持项目

如果 OmniProxy 对你的本地 AI 开发流程有帮助，欢迎点一个 Star。⭐

Star 会让更多有相同痛点的人看到这个项目，也能帮助后续功能优先级更清晰。也欢迎通过 Issue 提问题、提建议，或者直接提交 PR。真实使用场景里的反馈，比任何路线图都更有价值。

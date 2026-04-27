# OmniProxy

OmniProxy 是一个运行在本地的 AI API 令牌调度与无感代理网关。它把多个账号、多个厂商和多个客户端工具收拢到一个轻量桌面应用里，让 Codex、Claude Code 以及兼容 OpenAI / Anthropic 协议的请求可以自动选择可用账号、记录用量、在失败时切换，并在后台刷新额度。

如果你也经常在本地开发时遇到额度耗尽、账号切换、不同厂商配置分散、请求日志不可见这些问题，OmniProxy 试图把它们变成一个可控、可观察、可恢复的本地工作流。

## 为什么需要 OmniProxy

本地 AI 编程工具越来越多，但它们通常有几个共同痛点：

- 账号和 Token 分散在不同配置文件里，切换成本高。
- 单个账号触发 `429` 或临时上游错误时，客户端侧经常直接失败。
- 额度、请求历史、正在使用哪个账号不透明。
- OpenAI、Anthropic、DeepSeek、Kimi、Xiaomi MiMo 等入口的认证方式和 Base URL 各不相同。
- Codex / Claude Code 的本地配置修改容易出错，也不方便恢复。

OmniProxy 的思路很简单：在本机启动一个透明代理，把客户端请求先交给 OmniProxy，再由 OmniProxy 根据账号池状态转发到真实上游。

```text
Codex / Claude Code / API Client
              |
              v
     http://127.0.0.1:3000
              |
              v
        OmniProxy Gateway
      Token Pool + Scheduler
              |
              v
OpenAI / Anthropic / DeepSeek / Kimi / Xiaomi MiMo
```

## 当前能力

- 桌面端闭环：Wails + Go + Vue，启动后即可管理账号、代理、日志和设置。
- 本地透明代理：自动替换上游鉴权头，客户端无需感知实际使用的 Token。
- 多账号调度：支持队列模式与优先平衡使用模式，并避开正在请求中的账号。
- 自动重试切换：遇到 `429`、`502`、`503`、`504` 时可切换到下一个可用账号重试。
- 额度识别：解析 `x-ratelimit-remaining-tokens`、`x-ratelimit-remaining`、`x-ratelimit-remaining-requests`。
- Codex 额度刷新：Codex `auth.json` 账号在添加、启动和任务结束后会刷新订阅额度。
- 多厂商分组：OpenAI、Anthropic、DeepSeek、Kimi、Xiaomi MiMo 独立管理。
- Codex 支持：OpenAI 分组支持 API Key 和 Codex `auth.json`，兼容 `tokens.access_token` 与 `tokens.account_id`。
- Claude Code 快速接入：支持将 Claude Code 指向 DeepSeek、Kimi、Xiaomi MiMo 的本地路由。
- WebSocket 代理：Codex WebSocket 代理可在设置页开启或关闭，并记录请求用量。
- 实时日志：查看请求状态、耗时、使用账号和切换信息。
- 本地持久化：配置和账号数据写入本地 JSON 文件，关键状态立即保存，用量统计批量落盘。
- 安全边界：默认只监听 `127.0.0.1`，适合作为个人本地开发工具使用。

## 项目结构

```text
.
├── OmniProxyBackend/              # Wails 桌面主工程与 Go 后端
│   ├── internal/config/           # 本地配置、数据目录、默认值
│   ├── internal/logs/             # 内存日志记录器
│   ├── internal/proxy/            # 代理、路由、鉴权、用量解析、WebSocket
│   ├── internal/storage/          # JSON 本地存储
│   ├── internal/token/            # Token 池、调度、状态、统计
│   └── frontend/                  # 桌面应用内嵌 Vue 3 + Vite 前端
├── scripts/dev.ps1                # Wails 桌面开发启动脚本
└── README.md
```

默认端口和数据位置：

- 控制 API：`http://127.0.0.1:3890/api`
- 代理服务：`http://127.0.0.1:3000`
- 本地数据：默认写入 `%USERPROFILE%\.omniproxy`

## 快速开始

前置环境：

- Go
- Node.js
- Wails v2 CLI

开发运行：

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe dev
```

构建桌面应用：

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

构建产物：

```powershell
.\OmniProxyBackend\build\bin\OmniProxy.exe
```

## 使用方式

1. 启动 OmniProxy。
2. 在账号管理里添加需要使用的厂商账号。
3. 在全局设置里确认代理端口和各厂商 Base URL。
4. 启动本地代理。
5. 将 Codex、Claude Code 或其他 API 客户端指向本地代理地址。

常见代理地址：

```text
OpenAI compatible: http://127.0.0.1:3000
Codex backend:     http://127.0.0.1:3000/backend-api/codex
Claude router:     http://127.0.0.1:3000/anthropic-router
```

桌面应用内也提供了一键配置入口，可将本机 Codex 或 Claude Code 写入 OmniProxy 本地代理地址，并保留原始配置备份用于恢复。

## 支持的账号类型

| 厂商 | 凭据类型 | 说明 |
| --- | --- | --- |
| OpenAI | API Key | 使用 `Authorization: Bearer` |
| OpenAI / Codex | `auth.json` | 自动解析邮箱、access token、account id，并刷新 Codex 额度 |
| Anthropic | API Key | 使用 `x-api-key` |
| DeepSeek | API Key | 支持 OpenAI 兼容和 Anthropic 路由 |
| Kimi | API Key | 支持 Kimi Code 相关路由 |
| Xiaomi MiMo | API Key | 按量 API Key，通常以 `sk-` 开头 |
| Xiaomi MiMo | Token Plan | Token Plan Key，通常以 `tp-` 开头 |

## 控制 API

桌面前端优先通过 Wails 绑定调用后端。控制 API 仍保留给本地调试、外部脚本和兼容旧开发方式使用：

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

## 验证

后端测试：

```powershell
cd .\OmniProxyBackend
go test ./...
```

前端构建：

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

## 安全说明

OmniProxy 面向本地个人开发场景设计，默认只绑定 `127.0.0.1`。账号凭据保存在你选择的数据目录中，不会主动上传到任何第三方服务。

仍然建议：

- 不要把 `auth.json`、`.env`、`tokens.json` 或任何真实凭据提交到 Git。
- 不要把控制 API 暴露到公网或局域网。
- 使用前确认本地代理端口没有被其他程序占用。
- 分享日志或截图前检查是否包含账号名、路径或敏感配置。

## 路线图

- 继续收紧控制 API 的安全边界，并补充更细的本地访问控制。
- 拆分前端状态和页面结构，引入更清晰的组件边界。
- 增加请求历史图表、额度趋势和更细粒度的统计视图。
- 完善多厂商路由策略和鉴权策略。
- 增加 SSE、WebSocket、并发调度和异常恢复的端到端测试。
- 继续收敛桌面端调用链和发布流程，减少维护噪音。

## 参与贡献

欢迎一起把 OmniProxy 打磨成更顺手的本地 AI 网关。

如果你遇到了问题，欢迎提交 Issue，并尽量附上：

- 操作系统和运行方式。
- 你使用的客户端工具，例如 Codex、Claude Code 或自定义 API 客户端。
- 相关厂商、路由路径和错误日志。
- 预期行为和实际行为。

如果你想贡献代码，欢迎提交 Pull Request。比较适合优先参与的方向：

- 新厂商或新协议适配。
- 代理稳定性、并发调度、重试策略优化。
- 前端交互、可视化、配置体验改进。
- 测试覆盖、文档示例、问题复现用例。

提交 PR 前建议先跑：

```powershell
cd .\OmniProxyBackend
go test ./...
```

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

## 支持项目

如果 OmniProxy 对你的本地 AI 开发流程有帮助，欢迎点一个 Star。Star 会让更多有相同痛点的人看到这个项目，也能帮助后续功能优先级更清晰。

也欢迎通过 Issue 提问题、提建议，或者直接提交 PR。真实使用场景里的反馈，比任何路线图都更有价值。

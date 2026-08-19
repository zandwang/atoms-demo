# Atoms Demo 技术设计（Go 后端版）

> 状态：需求已确认，可按本文档进入本地开发。
>
> 关联文档：[需求分析计划](./REQUIREMENTS_ANALYSIS_PLAN.md)

## 1. 设计目标

构建一个本地可运行的 Atoms 风格 Web Demo。用户通过自然语言描述任意小型单页应用；Go 服务端调用真实模型，把需求转换为受约束的 HTML/CSS/JavaScript 文件集合，在隔离预览中执行。用户可继续迭代，并在刷新或重启本地服务后找回工作区、项目、对话、版本及预览运行数据。

最终交付物是一个 Go 可执行程序：启动后同时提供 API、SQLite 数据持久化和已构建的前端静态资源。Node.js 只在前端开发/构建阶段使用，不是运行时依赖。

```text
轻量初始化 → 创建项目 → 描述应用 → 模型生成 → 校验/编译 → 可交互预览
      ↑                                                        ↓
      └───── SQLite 持久化 ← 继续修改 ← 版本历史/回滚 ────────┘
```

## 2. 范围与边界

### 2.1 首版支持的应用范围

首版支持用户描述任意“小型单页应用”，例如计时器、计算器、表单、记账、计划、看板和数据展示工具。模型不再从固定模板列表中选择，而是返回最多三个受约束文件：`index.html`、`styles.css`、`app.js`。

不支持后端业务逻辑、数据库、登录、支付、外部网络请求、多页面路由、构建工具、第三方依赖或多人协作。为保证稳定性，限制文件数量、文件大小、执行能力和输入上下文；超出边界时明确拒绝或要求用户缩小范围。

### 2.2 已确认约束

- 每位用户在当前浏览器标签页中配置 OpenAI-compatible endpoint、model 和 API Key；服务端仅在单次请求内存中使用，不持久化或共享凭据。
- 首次使用以本地昵称创建工作区，不做邮箱注册。
- 首版先保证本地单二进制 Demo；用户已确认进行小规模线上验收，线上部署仍不承诺正式账号、跨设备同步或生产级持久化。
- 延展能力固定为“版本历史与回滚”。
- 前端采用 React，构建完成后由 Go 的 `embed` 打包；用户运行 Demo 时无需启动 Node.js。

## 3. 技术基线与选型

本仓库目前还没有 `go.mod`。新工程将声明 `go 1.26`，以本机 Go 1.26.4 工具链为目标；服务端优先使用现代标准库能力，例如增强 `http.ServeMux` 路由、`r.PathValue`、`http.NewResponseController`、`t.Context()` 和 `omitzero` JSON 标签。

| 层级 | 选择 | 原因 |
| --- | --- | --- |
| 服务端语言 | Go 1.26 | 单二进制、并发模型简单、适合本地 API/文件嵌入/SQLite 服务 |
| HTTP | 标准库 `net/http` | 使用方法 + 路径模式路由，避免不必要的 Web 框架依赖 |
| 数据库 | SQLite + `database/sql` + pure-Go driver | 真实持久化、事务与版本数据可靠；不要求 CGO 或额外服务 |
| 迁移 | 嵌入式 SQL migrations | 数据库 schema 随二进制发布，首次启动可自动初始化 |
| 模型接入 | OpenAI-compatible HTTP adapter | 兼容用户提供的 base URL / API Key，并可在测试中替换 |
| 前端 | React + TypeScript + Vite | 高效实现 Atoms 风格三栏交互；只在构建期需要 Node.js |
| 样式 | Tailwind CSS + 少量组件级样式 | 快速实现一致、响应式的产品界面 |
| 预览 | `iframe srcDoc` + sandbox + `postMessage` | 真实执行生成应用，同时隔离工作台和会话 Cookie |
| 服务端测试 | 标准库 `testing`、`httptest` | 无额外测试框架，使用临时 SQLite 和 fake model adapter |
| 前端/E2E 测试 | Vitest、Testing Library、Playwright | 覆盖组件和完整用户旅程 |

推荐 Go 的 SQLite driver 为 `modernc.org/sqlite`，以保留“本地一次构建即可运行”的体验。它是 Go 模块中的主要非标准库服务端依赖。

## 4. 总体架构

```text
┌──────────────────────────── 浏览器 ────────────────────────────┐
│  React 工作台（由 Go 提供静态资源）                               │
│  ├─ Onboarding / Project Sidebar / Agent Workspace               │
│  ├─ API Client（Cookie 会话）                                     │
│  ├─ Generation 状态机                                             │
│  └─ Preview iframe (sandbox="allow-scripts")                     │
│          ▲                    │ postMessage                      │
└──────────┼────────────────────┼─────────────────────────────────┘
           │ HTTP + SSE          │
           ▼                     │
┌────────────────────── Go 单体服务 ──────────────────────────────┐
│  cmd/atoms-demo                                                   │
│  ├─ Static Web Server（//go:embed all:dist）                      │
│  ├─ Session / Project / Version HTTP API                          │
│  ├─ Generation SSE Handler                                        │
│  ├─ Model Adapter → OpenAI-compatible API                         │
│  ├─ Generated Files Validator + Preview Artifact Builder          │
│  └─ SQLite Repository + Migrations                                │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS；请求内临时使用用户 API Key
                             ▼
                      配置的模型 API

┌──────────────────────── SQLite ─────────────────────────────────┐
│ workspaces · sessions · projects · messages · versions            │
│ preview_states · schema_migrations                                 │
└──────────────────────────────────────────────────────────────────┘
```

服务端是数据真相来源：前端不直接写浏览器数据库。这样可以让本地 Demo 的持久化、版本回滚、会话隔离和日后的线上迁移遵循同一套 API。

## 5. 核心设计：受约束文件生成与安全预览

模型不直接返回裸文本并立即执行。它返回满足严格 JSON 结构的 `AgentResult`，其中包含计划、用户可读说明和三个文件的受约束内容。Go 服务端对文件名、大小、危险 API、外部资源和 HTML 结构进行校验，再拼装为预览 artifact。

```text
用户需求
  → Go Agent Service
  → LLM 生成 { plan, assistantMessage, files }
  → JSON 解码 + 文件/安全校验
  → Preview Artifact Builder
  → CompiledArtifact { entryHtml, html, css, js, manifest }
  → 沙箱 iframe 预览 + 代码查看
```

这仍然提供真实可运行的代码和应用，而非静态截图；同时避免模型输出任意脚本、引入未知依赖，或通过生成代码读取工作台 Cookie、主页面 DOM 与服务端密钥。

工作台会显示三个层次：

1. Agent 计划：解释它将生成什么；
2. 文件摘要：生成文件、大小和校验结果，便于理解和迭代；
3. 已编译代码：实际在预览中运行的 HTML/CSS/JavaScript。

## 6. 领域模型与 SQLite 持久化

### 6.1 Go 领域对象

服务端领域对象使用强类型结构体；来自模型的 JSON 以严格模式解码，不接受未知字段。所有数据库时间以 UTC RFC3339 格式保存，API 响应以 JSON 返回。

```go
type AppTemplate string

const (
	TemplateCustom AppTemplate = "custom"
)

type GeneratedFiles struct {
	HTML string `json:"indexHtml"`
	CSS  string `json:"stylesCss"`
	JS   string `json:"appJs"`
}

type AppSpec struct {
	Files *GeneratedFiles
}

type Project struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"-"`
	Name            string    `json:"name"`
	Summary         string    `json:"summary"`
	ActiveVersionID *string   `json:"activeVersionId,omitzero"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type GenerationVersion struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"projectId"`
	ParentVersionID *string       `json:"parentVersionId,omitzero"`
	Sequence        int           `json:"sequence"`
	Request         string        `json:"request"`
	Plan            AgentPlan     `json:"plan"`
	Spec            AppSpec       `json:"spec"`
	Artifact        CompiledArtifact `json:"artifact"`
	CreatedAt       time.Time     `json:"createdAt"`
}
```

`AppSpec` 对外编码为 `{schemaVersion: 1, template: "custom", files: {...}}`。代码中暂时保留旧版 `TodoSpec`、`NotesSpec`、`HabitsSpec` 的反序列化与编译能力，仅用于读取现有 SQLite 历史版本；模型 prompt 和新生成链路只产生 `custom` 文件规格。完成数据迁移前不能删除这层兼容代码。

### 6.2 数据库表

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `workspaces` | `id`, `display_name`, timestamps | 轻量本地用户工作区 |
| `sessions` | `token_hash`, `workspace_id`, `expires_at` | 浏览器 Cookie 对应的随机、不透明会话 token |
| `projects` | `id`, `workspace_id`, `name`, `summary`, `active_version_id` | 项目元数据与当前预览版本 |
| `messages` | `id`, `project_id`, `role`, `content`, `generation_id` | 用户、Agent、系统反馈的对话记录 |
| `generation_attempts` | `id`, `project_id`, `status`, `error_code`, timestamps | 保留生成中断、失败和重试审计信息 |
| `versions` | `id`, `project_id`, `parent_version_id`, `sequence`, `spec_json`, artifact | 每次成功生成的不可变快照 |
| `preview_states` | `project_id`, `version_id`, `state_json`, `updated_at` | 预览中用户实际操作产生的运行态数据 |
| `schema_migrations` | `version`, `applied_at` | 已执行迁移记录 |

数据库启动规则：

- 文件默认位于 `./data/atoms-demo.db`，可用 `ATOMS_DATA_DIR` 调整目录；
- 开启 `PRAGMA foreign_keys = ON`、WAL journal mode、合理的 busy timeout；
- 所有 migrations 使用 `embed.FS` 随二进制发布并在启动时顺序执行；
- 删除项目在一个数据库 transaction 中级联删除消息、尝试、版本和预览状态；
- 回滚只更新 `projects.active_version_id` 并追加 system message，不删除历史版本。

### 6.3 本地会话与初始化

首次访问时，前端显示昵称输入。`POST /api/session/initialize` 创建 workspace 和随机 256-bit session token；服务端只在数据库保存 token 的 SHA-256 哈希，在浏览器写入 `HttpOnly`、`SameSite=Lax` 的会话 Cookie。

后续请求由 session middleware 将 workspace 写入 `context.Context`。所有项目、版本和运行态查询都带上 workspace 条件，避免同一台机器的不同浏览器 profile 相互看到数据。此机制不是生产级账户系统，但足以满足 Demo 的“初始化/注册”流程且无需邮箱。

## 7. 模型与 Agent 生成链路

### 7.1 模型配置与环境变量契约

```dotenv
ATOMS_DATA_DIR=./data
ATOMS_MODEL_TIMEOUT=120s
# 仅本地/受信环境需要连接 localhost、私网或内网自部署模型时启用
ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS=false
```

每位用户在前端自行填写 OpenAI-compatible `endpoint`、`model` 和 `API Key`；三项仅保存在当前标签页的 `sessionStorage`，每次生成时分别通过 `X-Model-Base-URL`、`X-Model-Name`、`X-Model-API-Key` 请求头发送给 Go 服务端。服务端只在该次请求的内存中使用它们，不写入 SQLite、Cookie、日志或响应。关闭标签页后浏览器自动清除配置。

endpoint 必须是绝对 `http`/`https` URL，不能包含 userinfo、query 或 fragment；默认拒绝 localhost、回环、链路本地、私网和保留地址，服务端通过 `ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS=true` 才允许受信本地/内网部署。服务端不使用代理环境变量访问用户 endpoint，以减少绕过地址校验的风险。健康检查只报告服务是否可接受 BYOK 请求，绝不输出 endpoint、请求头或上游认证响应内容。`.env` 不提交；另提供不含真实值的 `.env.example`。

通用应用需要模型返回完整 HTML/CSS/JavaScript，响应通常比旧规格更大，因此模型调用默认超时为 120 秒。部署者可通过 `ATOMS_MODEL_TIMEOUT` 使用 Go duration 格式在 10 秒到 10 分钟之间调整；非法值应使服务启动失败，而不是静默回退。

### 7.2 HTTP API

所有 `/api/` 路由使用增强的标准库路由模式，例如：

```text
GET    /api/session
POST   /api/session/initialize

GET    /api/projects
POST   /api/projects
GET    /api/projects/{projectID}
PATCH  /api/projects/{projectID}
DELETE /api/projects/{projectID}

GET    /api/projects/{projectID}/messages
GET    /api/projects/{projectID}/versions
POST   /api/projects/{projectID}/generate          (SSE; requires X-Model-Base-URL, X-Model-Name, X-Model-API-Key)
POST   /api/projects/{projectID}/versions/{versionID}/activate

GET    /api/projects/{projectID}/versions/{versionID}/preview-state
PUT    /api/projects/{projectID}/versions/{versionID}/preview-state
```

`projectID` 和 `versionID` 由 `r.PathValue(...)` 读取，并在 repository 层以当前 workspace 范围校验。所有非 SSE 请求返回统一 envelope：

```json
{ "data": {}, "error": null }
```

失败响应为：

```json
{ "data": null, "error": { "code": "MODEL_CONFIG_REQUIRED", "message": "…", "retryable": false } }
```

### 7.3 生成请求与 SSE 协议

请求体只带本次用户文字；服务端自行从 SQLite 读取当前版本和最多 8 条最近对话，避免客户端伪造版本上下文。

```json
{
  "userRequest": "做一个可开始、暂停和重置的番茄钟"
}
```

`POST /api/projects/{projectID}/generate` 返回 `text/event-stream`：

| 事件 | 负载 | 触发时机 |
| --- | --- | --- |
| `stage` | `{ "status", "label" }` | 真实开始模型请求、验证或编译时 |
| `result` | `{ "version", "message" }` | 数据库 transaction 成功提交后 |
| `error` | `{ "code", "message", "retryable" }` | 可预期失败时 |

处理步骤：

1. session middleware 和项目归属校验；校验 `userRequest` 为 1–2,000 字符。
2. 在 transaction 中保存 user message 与 `generation_attempts(status=running)`。
3. 写入 `requesting_model` 阶段，创建具有超时原因的请求 context 后调用模型。
4. 写入 `validating` 阶段，对模型 JSON 和业务规则做完整校验。
5. 写入 `compiling` 阶段，调用纯 Go 编译器生成 artifact 与 checksum。
6. 在一个 transaction 中写入 assistant message、version、attempt 状态，更新 `active_version_id`。
7. 刷新并发送 `result`；若中途失败，更新 attempt、写入 system message，并发送 `error`。

SSE handler 使用 `http.NewResponseController(w).Flush()` 及时发送真正的阶段事件。它不伪造“思考进度”；每个状态对应服务端实际正在执行的工作。

### 7.4 Agent 结果与验证

模型的唯一有效输出为：

```json
{
  "plan": {
    "summary": "…",
    "steps": ["…", "…"]
  },
  "assistantMessage": "…",
  "appSpec": {
    "schemaVersion": 1,
    "template": "custom",
    "files": {
      "indexHtml": "<main>…</main>",
      "stylesCss": "body { … }",
      "appJs": "document.…"
    }
  }
}
```

模型适配器优先请求 JSON object 模式；若配置的兼容接口不支持该参数，则重试普通 Chat Completions，并始终以 `json.Decoder.DisallowUnknownFields`、大小限制和二次业务校验拒绝额外或错误字段。不要接受 Markdown 围栏、JSON 之外的裸文本或未校验的 JSON Patch。

每个 `AppSpec` 的校验至少覆盖：

- `schemaVersion` 固定为 `1`；
- `template` 固定为 `custom`，文件名固定为 `index.html`、`styles.css`、`app.js` 对应的三个字段；
- HTML 必填且不超过 60 KiB，CSS 不超过 60 KiB，JavaScript 不超过 100 KiB；
- 拒绝外部脚本/样式、iframe、网络 API、Cookie、父窗口导航以及可逃逸 `<style>`/`<script>` 容器的内容；
- 不支持后端、数据库、登录、支付、多页面、第三方依赖、外部资源或网络请求；
- 迭代请求携带当前完整文件并返回完整新文件，以当前版本作为 `parent_version_id`，不接受 patch；
- 超出能力边界的需求应在模型回复中解释并收敛，若输出仍不合法则返回 `MODEL_OUTPUT_INVALID`。

### 7.5 Model Adapter

```go
type ModelAdapter interface {
	Generate(ctx context.Context, input AgentPromptInput) (AgentResult, error)
}
```

首版实现 `OpenAICompatibleAdapter`，基于 `net/http` 访问 `${userBaseURL}/chat/completions`。生产 handler 为每次生成读取并校验三个用户请求头，临时构造 adapter，并在请求结束后释放引用；adapter 只在 Go 服务端依赖图中存在。测试 handler 仍可显式注入 fake adapter，且不依赖真实配置。

测试和本地开发使用 `FakeModelAdapter` 注入固定结果，覆盖成功、超范围、非法 JSON、认证失败和超时。fake 仅供测试或明确的开发开关使用，绝不能在真实模型失败后静默地把伪结果伪装为 Agent 输出。

### 7.6 Prompt 约束

System Prompt 必须：

- 说明通用小型单页应用的能力和非目标；
- 要求只返回已定义 JSON 结构，其中 HTML/CSS/JavaScript 分别放入固定文件字段；
- 要求超范围需求缩小为可离线运行的小型单页应用；
- 禁止后端、外部 URL、网络 API、第三方依赖、密钥或声称运行了不存在的工具；
- 在迭代模式根据当前 `AppSpec` 输出完整新文件，而不是局部 patch；
- 使用简短中文撰写面向用户的计划和说明。

用户输入、项目名、历史对话和当前应用文本始终是不可信上下文，不能覆盖系统边界。模型结果必须再次经过服务端文件与能力校验。

## 8. Go 编译器与安全预览

### 8.1 `CompiledArtifact`

```go
type CompiledArtifact struct {
	EntryHTML string           `json:"entryHtml"`
	HTML      string           `json:"html"`
	CSS       string           `json:"css"`
	JS        string           `json:"js"`
	Manifest  ArtifactManifest `json:"manifest"`
}
```

新生成链路由 `CompileGenerated` 纯函数构成。它在文件校验通过后注入 CSP、状态桥接脚本和生成的 HTML/CSS/JavaScript，得到完整 `srcDoc`。旧版三个固定模板编译器只为历史版本兼容保留，不再由模型选择。

每个 artifact 都包含：

- `entryHtml`：完整 `srcDoc`，供 iframe 直接加载；
- `html`、`css`、`js`：供“代码”面板查看；
- `manifest`：产物类型、通用交互标记和 SHA-256 checksum。

预览中的功能由本次模型生成的客户端逻辑真实驱动，不是截图、录制动画或从固定业务模板中选择。

### 8.2 iframe 隔离与状态桥接

```text
Parent → iframe: preview:init { versionId, state }
iframe → Parent: preview:ready { versionId }
iframe → Parent: preview:state-change { versionId, state }
Parent → iframe: preview:restore { versionId, state }
```

- iframe 使用 `sandbox="allow-scripts"`，不授予 `allow-same-origin`、表单、弹窗、下载或导航权限。
- artifact 无外链脚本或网络请求，不能访问 parent DOM；生成脚本仍按不可信代码对待。
- iframe 只通过 `postMessage` 回传不超过 64 KiB 的 JSON object 运行态；前端验证 `event.source` 和 `versionId` 后，Go API 再验证对象与项目归属并持久化。
- Go API 用当前 session 和项目归属再次验证，再写入 `preview_states`。
- 重新打开项目或切换版本时，前端从 API 读取状态并回传 iframe 恢复。
- `srcDoc` 追加 CSP：只允许内联样式、内联脚本和 data URL 图片，不允许网络连接、frame 嵌套或外部资源。

这让生成应用主动调用 `window.atomsPreview.publish(state)` 时可以持久化自身运行态，同时它无法接触 Cookie、SQLite、API Key 或主工作台的同源内容。当前危险能力检查是 Demo 级字符串校验，不是完整 JavaScript 静态分析；opaque-origin iframe sandbox 与 CSP 才是主要运行时边界。

## 9. React 前端设计与 Go 静态嵌入

### 9.1 信息架构

```text
/
├─ 未初始化：Welcome / 输入昵称
└─ 已初始化：Workspace
   ├─ 左栏：品牌、创建项目、项目列表、当前用户
   ├─ 中栏：项目标题、对话记录、Agent 阶段、提示词输入区
   └─ 右栏：预览 / 代码 / 文件 / 版本历史
```

桌面端为三栏布局。窄屏时项目栏保留，中央工作区与右栏内容折叠为标签页；首次初始化、创建项目、生成、预览、迭代与回滚在移动宽度下仍可完成。

### 9.2 前端职责

前端只负责展示和交互，不直接保存业务数据：

- 启动时请求 `/api/session` 决定显示 Welcome 或 Workspace；
- 所有项目、消息、版本和预览状态由 Go API 读取和写入；
- 通过 `fetch()` 消费生成 API 的 POST SSE 流，更新短暂的生成状态；
- 收到 `result` 后重新拉取该项目/版本，避免客户端自行拼装持久化对象；
- iframe 的运行态通过 API 写入，而不是 `localStorage`；
- API 使用同源 Cookie；模型 endpoint、model 和 API Key 只保存在当前标签页 `sessionStorage`，生成时随请求发送。

### 9.3 关键组件

```text
web/src/
├─ app/App.tsx
├─ api/client.ts
├─ api/generation-stream.ts
├─ features/
│  ├─ onboarding/WelcomeForm.tsx
│  ├─ projects/ProjectSidebar.tsx
│  ├─ projects/NewProjectDialog.tsx
│  ├─ workspace/AgentTimeline.tsx
│  ├─ workspace/PromptComposer.tsx
│  ├─ workspace/GenerationStatus.tsx
│  ├─ preview/PreviewPanel.tsx
│  ├─ preview/SandboxFrame.tsx
│  ├─ preview/CodeViewer.tsx
│  └─ versions/VersionHistory.tsx
├─ hooks/
├─ types/api.ts
└─ styles/
```

### 9.4 生成状态机

```text
idle
  └─ submit → requesting_model
                  └─ stage → validating → compiling → ready
                  └─ error → failed
failed ─ retry → requesting_model
ready  ─ submit iteration → requesting_model
```

- 同一项目处于生成状态时禁用重复提交，但可以浏览其他项目与历史版本。
- 切换项目不会取消已发请求；服务端按项目和 session 写入，结果返回后前端仅刷新对应项目。
- 每个 request 有 `generationID`；晚到结果只能追加版本，不能覆写用户之后主动恢复的 `activeVersionID`。
- 刷新或关闭前若请求未结束，`generation_attempts` 保留 running 状态；服务端启动时将遗留请求标为 interrupted，前端显示“上次生成未完成，可重试”。

### 9.5 开发与运行方式

开发期可分别运行 Go API 与 Vite；Vite 通过代理把 `/api` 转给 Go 服务，保持同源 Cookie 行为。构建期 Vite 输出到 `internal/webembed/dist`，再由同目录中的 `//go:embed all:dist` 编入 Go 二进制；这样不触发 `go:embed` 不能跨包目录引用资源的限制。

```text
开发：make dev-api       # Go API
      make dev-web       # Vite 前端，代理 API

构建：make build         # 构建 internal/webembed/dist，再 go build
运行：./bin/atoms-demo   # 单个 Go 可执行程序，无 Node 运行时
```

`make` 目标和 README 将封装具体命令；用户只需使用最终的 Go 启动命令进行本地验收。

## 10. 源码目录设计

```text
.
├─ cmd/
│  └─ atoms-demo/
│     └─ main.go
├─ internal/
│  ├─ app/                 # 依赖装配、生命周期、HTTP server
│  │  ├─ server.go
│  │  ├─ session.go
│  │  ├─ projects.go
│  │  ├─ history.go
│  │  ├─ generation.go
│  │  └─ response.go
│  ├─ config/              # .env / OS 环境读取与校验
│  ├─ domain/              # 领域类型与校验规则
│  ├─ agent/
│  │  ├─ adapter.go
│  │  ├─ openai_compatible.go
│  │  ├─ fake.go
│  │  └─ prompt.go
│  ├─ compiler/
│  │  ├─ compile.go
│  │  ├─ generic.go          # 新生成链路
│  │  ├─ todo.go             # 旧版本兼容
│  │  ├─ notes.go            # 旧版本兼容
│  │  └─ habits.go           # 旧版本兼容
│  ├─ store/
│  │  ├─ repository.go
│  │  ├─ sqlite/
│  │  │  ├─ store.go
│  │  │  ├─ projects.go
│  │  │  ├─ workspaces.go
│  │  │  ├─ generations.go
│  │  │  └─ migrations.go
│  │  └─ migrations/
│  └─ webembed/
│     ├─ static.go          # //go:embed all:dist
│     └─ dist/              # Vite 构建产物；由 web/vite.config.ts 输出
├─ web/                     # React + TypeScript + Vite，仅构建期需要 Node
│  ├─ go.mod                # 仅作 Go 工具边界，避免扫描 node_modules
│  ├─ src/
│  ├─ public/
│  ├─ package.json
│  └─ vite.config.ts
├─ tests/
│  └─ e2e/
├─ .env.example
├─ Makefile
├─ go.mod
└─ README.md
```

依赖方向必须单向：

```text
app → agent / compiler / domain / store interfaces
sqlite store → domain
compiler → domain
web → HTTP API（不导入 Go 内部实现）
```

`compiler` 不依赖 HTTP、数据库、模型或 React；`agent` 不直接写数据库；handler 只处理 HTTP 编解码与状态码映射。这保证每层可独立测试。

## 11. 错误处理、可靠性与安全

| 错误代码 | 场景 | 用户体验 | 是否可重试 |
| --- | --- | --- | --- |
| `API_KEY_REQUIRED` | 生成请求未携带用户 Key | 提示用户在当前标签页设置 Key | 设置后重试 |
| `MODEL_CONFIG_REQUIRED` | 生成请求未携带 endpoint 或 model | 提示补充模型配置 | 设置后重试 |
| `MODEL_ENDPOINT_INVALID` | endpoint 格式不合法或命中私网限制 | 提示检查 endpoint | 修改后重试 |
| `AUTH_ERROR` | 上游拒绝用户凭据 | 提示检查用户自己的 API Key | 更新后重试 |
| `UPSTREAM_TIMEOUT` | 模型超时/网络失败 | 保留请求和项目，提供重试 | 是 |
| `MODEL_OUTPUT_INVALID` | 结果不符合受控规格 | 告知未生成可用应用 | 是 |
| `UNSUPPORTED_REQUEST` | 需求超出小型离线单页应用边界 | 解释当前范围并建议缩小需求 | 是 |
| `PREVIEW_STATE_INVALID` | iframe 回传非法状态 | 不保存非法数据，提示刷新预览 | 是 |
| `NOT_FOUND` / `FORBIDDEN` | ID 不存在或不属于当前工作区 | 返回通用错误，避免数据泄漏 | 视情况 |

可靠性与安全规则：

- 用户 endpoint、model、API Key、授权头、Cookie token 和上游原始响应不得写入日志、错误消息或数据库；三项模型配置只允许存在于当前标签页 `sessionStorage` 和单次 Go 请求内存中。
- 用户 endpoint 必须通过 scheme、userinfo、query、fragment、主机地址和 DNS 解析校验；默认拒绝 loopback、私网、链路本地、未指定和保留地址。允许私网时必须由部署者显式打开 `ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS`，且不建议在公开服务启用。
- 模型请求使用 `context.WithTimeoutCause`；错误判断使用 `errors.Is` / `errors.AsType`，保留根因但不向用户暴露敏感细节。
- 生成链路记录项目/尝试 ID、阶段、耗时、错误码、上游 HTTP 状态和脱敏网络原因，不记录用户需求正文、endpoint、model、凭据或上游响应体。
- 生成 API 限制请求体、字符串长度、会话上下文数量和并发数；同一 session 同时只允许一个运行中的生成。
- 所有数据库写入使用参数化查询；所有归属查询都绑定 workspace ID。
- 删除项目需要前端确认；删除本地 SQLite 数据不可恢复，版本回滚永不删除历史。
- Web 静态资源、API 和 iframe 均由同源 Go 服务提供，默认不启用宽松 CORS。

## 12. 测试设计

### 12.1 Go 单元与集成测试

- `domain`：custom 文件协议的严格输入、未知字段、超长文件、危险能力、错误版本与通用运行态；旧模板兼容仍有回归测试。
- `compiler`：生成文件编译为完整 artifact、checksum 稳定、无外部脚本与网络调用。
- `agent`：OpenAI 请求构造、JSON 解码、错误映射、structured-output 回退；使用 mock `http.RoundTripper`，绝不访问真实 Key。
- `store/sqlite`：migration、workspace 隔离、项目 CRUD、保存版本、回滚、级联删除与运行态恢复。
- `app`：使用 `httptest` 覆盖 Cookie 会话、路径参数、JSON 错误 envelope、SSE 事件顺序、版本恢复和预览状态 API。

测试使用 `t.Context()` 作为上下文来源，临时数据库存放在 `t.TempDir()`。有 context 的并发测试应尊重取消原因和 deadline。

### 12.2 前端与 E2E 测试

1. 首次进入输入昵称，创建项目。
2. 使用 fake model adapter 输入“做一个番茄钟”，得到可交互预览。
3. 在 iframe 新增、完成、删除项目；刷新页面后状态仍恢复。
4. 迭代当前项目，版本数增加且新版本成为 active version。
5. 从历史版本回滚，预览和 active version 一致。
6. 模型超时或非法输出时，原始用户请求仍可见，重试可操作。
7. 以构建后的单二进制启动，验证 `/`、`/api/session` 和前端路由均可访问。

真实模型调用只进入人工本地验收脚本，不作为 CI 必需条件。

## 13. 开发顺序与验收检查点

| 阶段 | 交付内容 | 完成条件 |
| --- | --- | --- |
| 1. Go 工程骨架 | `go.mod`、配置、`net/http`、健康检查、SQLite migration、React/Vite shell | Go API 可启动；缺配置时给出明确提示 |
| 2. 本地工作区 | session Cookie、Onboarding、项目 CRUD、SQLite repository | 重启服务和刷新浏览器后昵称、项目仍存在 |
| 3. 生成最短链路 | Agent adapter、SSE、通用文件编译、iframe 预览 | 一条真实模型请求可生成任意受约束小型 SPA 与代码 |
| 4. 通用迭代 | 当前完整文件、文件安全校验、错误处理 | 可基于当前应用自然语言修改；失败请求可重试 |
| 5. 版本与运行态 | 版本历史、回滚、iframe 状态桥接 | 刷新、重启、回滚后数据一致 |
| 6. 打磨与测试 | 响应式界面、Go/前端测试、README、验收脚本 | P0/P1 用例全部通过，`make build` 产出单二进制 |

应在阶段 3 完成后立即做一次真实 API 联调，确认用户提供的 endpoint、model、Key 和兼容接口能力，而不是在所有 UI 完成后才发现配置问题。

## 14. 需求追踪

| 原始需求 | Go 版设计落点 |
| --- | --- |
| 可运行 Atoms Demo | Go 单体服务 + 嵌入式 React 静态资源 + 本地启动流程 |
| 智能体驱动代码/应用生成 | Go Agent Service → `AppSpec` → `CompiledArtifact` |
| 可视化网页展示 | sandbox iframe 运行生成应用；预览/代码/文件切换 |
| 真实交互 | 工作台会话、真实 SSE 阶段、预览内 CRUD/搜索/打卡 |
| 数据持久化 | SQLite 保存工作区、项目、消息、尝试、版本及预览运行态 |
| 初始化/注册与核心流程 | 昵称 onboarding → Cookie session → 项目 → 生成 → 迭代 |
| 延展能力 | 版本历史与回滚 |
| 可交付性 | 单 Go 二进制、环境变量契约、测试策略、README 与本地验收计划 |

## 15. 开始编码前的完成定义

- [x] 需求范围、非目标和本地交付边界已确认。
- [x] Go 后端、React 前端构建/嵌入及单二进制运行方式已确认。
- [x] 数据所有权、SQLite schema、会话与版本策略已定义。
- [x] BYOK endpoint/model/API Key 边界、生成规格、编译器和预览隔离已定义。
- [x] HTTP API、SSE、错误码和测试路径已定义。
- [x] 初始化 `go.mod`、`web/`、`.env.example`、`.gitignore`、migrations 和基础脚本。
- [ ] 使用用户自行提供的 Key 完成一次真实 API 最短链路联调。

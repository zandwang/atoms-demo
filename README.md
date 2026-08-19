# Atoms Demo

一个 Atoms 风格的应用生成 Demo：用户用自然语言描述任意受支持的小型单页应用，Go 服务端调用 OpenAI-compatible 模型生成受约束的 HTML、CSS 和 JavaScript，并在隔离环境中运行。用户可以继续提出修改要求，每次成功生成都会保存为新版本。React 工作台、SQLite 数据库和静态资源都由一个 Go 二进制提供。

当前已实现：

- 本地昵称工作区与 HttpOnly Cookie 会话；
- 项目创建、改名、删除及 SQLite 持久化；
- 真实模型生成链路：严格 JSON 文件协议、阶段 SSE、错误保留与重试入口；
- 通用小型单页应用：不限定业务模板，可生成计时器、计算器、表单、计划、看板等离线工具；
- sandbox iframe 预览、HTML/CSS/JavaScript 和文件协议查看；
- 生成版本历史、版本恢复，以及预览内交互状态的本地保存与恢复。

## 前置条件

- Go 1.26 或更高版本
- Node.js 22 或更高版本、npm（仅用于前端构建）
- 一个支持 Chat Completions 的 OpenAI-compatible 模型接口

## 配置与启动

1. 以 `.env.example` 为参考创建 `.env`。不要提交 `.env`。模型 endpoint、model 和 API Key 都由每位用户在浏览器中自行设置，不放入 `.env`。

   ```dotenv
   ATOMS_ADDR=:8080
   ATOMS_DATA_DIR=./data
   ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS=false
   ```

2. 安装前端构建依赖：

   ```sh
   make web-install
   ```

3. 构建并启动单个 Go 可执行程序：

   ```sh
   make run
   ```

4. 在浏览器访问 <http://localhost:8080>，输入昵称并在右上角设置自己的 endpoint、model 和 API Key，然后创建项目并描述一个小型单页应用，例如“做一个可开始、暂停和重置的番茄钟”。三项模型配置仅保存在当前浏览器标签页中。

构建后的 `bin/atoms-demo` 不需要 Node.js 运行。默认数据库位于 `./data/atoms-demo.db`；可使用 `ATOMS_DATA_DIR` 指定其他本地目录，使用 `ATOMS_ADDR` 修改监听地址。
部署平台提供 `PORT` 且未设置 `ATOMS_ADDR` 时，服务会自动监听 `:${PORT}`。

## 验证

```sh
go test ./...
cd web && npm run build
make build
```

测试覆盖生成文件校验、模型协议回退、SSE、SQLite 隔离、通用应用版本恢复和预览状态校验。真实模型调用使用用户请求头中的 API Key，不会出现在自动测试中；服务端不会保存或返回 API Key。

## 模型调用排查

生成请求会输出脱敏的结构化日志，包括 `generation started`、`model generation started`、`model generation completed/failed` 和最终 `generation completed/failed`。日志带有 `project_id`、`attempt_id`、耗时、错误码、上游 HTTP 状态和安全归类后的网络原因，便于串联一次请求。

模型调用默认超时为 120 秒，可用 `ATOMS_MODEL_TIMEOUT=2m` 调整（允许 10 秒到 10 分钟）。看到 `UPSTREAM_TIMEOUT` 时，先查同一个 `attempt_id` 的 `model generation started` 与 `model generation failed`。日志不会记录用户需求正文、endpoint、model、API Key、Authorization、Cookie 或上游响应体。

## 安全与范围

模型只能返回 `index.html`、`styles.css`、`app.js` 三个文件的严格 JSON 协议。服务端限制文件大小，拒绝外部脚本、外部样式、iframe、网络 API、Cookie 和逃逸内联容器的内容，再拼装预览 artifact。预览 iframe 使用 `sandbox="allow-scripts"` 和无网络 CSP，运行态只能通过受验证的 `postMessage` 发送回本地 API。

这是 Demo 级隔离，不是完整的 JavaScript 静态分析或生产级恶意代码执行平台。主要运行时边界是 opaque-origin iframe sandbox 与 CSP；公开服务仍需要增加限流、资源配额和更强的内容审查。单个生成版本最多包含约 60 KiB HTML、60 KiB CSS 和 100 KiB JavaScript，不支持后端、数据库、登录、支付、多页面、第三方依赖、外部资源或网络请求。旧版 `todo`、`notes`、`habits` 规格仅为读取已有 SQLite 历史版本保留，新生成不会再选择这些模板。

当前仍使用匿名 workspace，不含真实邮箱注册、登录恢复、公开分享、多设备同步、支付或任意多页应用。每位用户自行提供模型 endpoint、model 和 API Key；三项配置只在当前标签页和单次生成请求中使用，服务端不保存。服务端默认拒绝私网 endpoint，公开部署时不要打开私网放行开关。版本数据和预览状态保存在服务端 SQLite，删除项目不可恢复；公开部署时需要持久化磁盘或迁移到托管数据库。

## 项目文档

- [需求分析计划](./REQUIREMENTS_ANALYSIS_PLAN.md)
- [技术设计（Go 后端）](./CODE_DESIGN.md)
- [开发进度台账](./DEVELOPMENT_PROGRESS.md)

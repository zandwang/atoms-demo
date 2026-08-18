# Atoms Demo

一个本地运行的 Atoms 风格应用生成 Demo：用户用自然语言描述小型应用，Go 服务端调用 OpenAI-compatible 模型生成受控规格，再确定性地编译为可运行的 HTML、CSS 和 JavaScript。React 工作台、SQLite 数据库和静态资源都由一个 Go 二进制提供。

当前已实现：

- 本地昵称工作区与 HttpOnly Cookie 会话；
- 项目创建、改名、删除及 SQLite 持久化；
- 真实模型生成链路：受控 JSON 规格、阶段 SSE、错误保留与重试入口；
- 三类可运行应用：待办清单、笔记板、习惯打卡；
- sandbox iframe 预览、编译代码/规格查看；
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

4. 在浏览器访问 <http://localhost:8080>，输入昵称并在右上角设置自己的 endpoint、model 和 API Key，然后创建项目并描述一个待办、笔记或习惯打卡应用。三项配置仅保存在当前浏览器标签页中。

构建后的 `bin/atoms-demo` 不需要 Node.js 运行。默认数据库位于 `./data/atoms-demo.db`；可使用 `ATOMS_DATA_DIR` 指定其他本地目录，使用 `ATOMS_ADDR` 修改监听地址。

## 验证

```sh
go test ./...
cd web && npm run build
make build
```

测试覆盖规格校验、模型协议回退、SSE、SQLite 隔离、版本恢复和预览状态校验。真实模型调用使用用户请求头中的 API Key，不会出现在自动测试中；服务端不会保存或返回 API Key。

## 安全与范围

模型不能提交任意前端源码。它只能返回 `todo`、`notes` 或 `habits` 的严格规格，Go 编译器使用固定模板生成预览。预览 iframe 使用 `sandbox="allow-scripts"` 和无网络 CSP，运行态只能通过受验证的 `postMessage` 发送回本地 API。

当前仍使用匿名 workspace，不含真实邮箱注册、登录恢复、公开分享、多设备同步、支付或任意多页应用。每位用户自行提供模型 endpoint、model 和 API Key；三项配置只在当前标签页和单次生成请求中使用，服务端不保存。服务端默认拒绝私网 endpoint，公开部署时不要打开私网放行开关。版本数据和预览状态保存在服务端 SQLite，删除项目不可恢复；公开部署时需要持久化磁盘或迁移到托管数据库。

## 项目文档

- [需求分析计划](./REQUIREMENTS_ANALYSIS_PLAN.md)
- [技术设计（Go 后端）](./CODE_DESIGN.md)
- [开发进度台账](./DEVELOPMENT_PROGRESS.md)

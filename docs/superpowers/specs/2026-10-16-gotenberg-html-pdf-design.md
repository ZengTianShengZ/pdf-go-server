# Gotenberg HTML→PDF 服务设计

**日期：** 2026-10-16  
**状态：** 已批准（对话确认）  
**范围：** 将微信云托管 Go 计数器模板改造成基于 Gotenberg 的「一容器」HTML→PDF 服务

## 背景

现有仓库是微信云托管 Go 模板（计数器 + MySQL），与 PDF 无关。目标是提供对外 HTTP 接口：用本地 HTML 模板 + 请求 JSON 数据生成 PDF，转换能力由 [Gotenberg](https://github.com/gotenberg/gotenberg)（Chromium）承担。

## 目标

- **一容器可用**：镜像内同时运行 Gotenberg（仅本机）与 Go API（对外）
- **单一对外接口**：`POST /api/pdf`，JSON 为模板数据，返回 PDF
- **固定模板**：`template/v1-home-inspection.html`（通过 `window.__REPORT_DATA__` + 页面 JS 填 DOM）
- **暂不鉴权**

## 非目标

- 多模板路由 / 模板热更新
- 鉴权、配额、异步任务队列
- LibreOffice / Office 文档转换
- 保留原计数器与 MySQL

## 架构

```
客户端
  │  POST /api/pdf  { JSON 业务数据 }
  ▼
Go HTTP 服务 (:80)          ← 唯一对外端口
  │  1. 读 template/v1-home-inspection.html
  │  2. 把 JSON 注入 window.__REPORT_DATA__
  │  3. multipart POST → Gotenberg
  ▼
Gotenberg (127.0.0.1:3000)  ← 仅容器内访问
  │  Chromium 渲染 HTML（执行模板内 JS）
  ▼
返回 application/pdf 给客户端
```

容器内由 entrypoint 管理双进程：先启动 Gotenberg 绑定 `127.0.0.1:3000`，再前台运行 Go 监听 `:80`。

## API

### `POST /api/pdf`

| 项 | 约定 |
|---|---|
| Content-Type | `application/json` |
| Body | 任意 JSON 对象（字段与模板约定一致，如 `name`、`email`、`phone`、`address`、`city`、`state`、`zip`、`country`） |
| 成功 | `200`，`Content-Type: application/pdf`，body 为 PDF 二进制 |
| 失败 | `4xx/5xx` + JSON：`{ "code": <int>, "message": "<string>" }` |

### `GET /health`

**必须实现。** 探测本机 Gotenberg（例如请求其 health 端点）：

- 可达：`200` `{ "status": "ok" }`
- 不可达：`503` `{ "status": "unavailable", "message": "gotenberg unreachable" }`

### 错误映射

| 情况 | HTTP |
|---|---|
| Body 非合法 JSON，或根值不是 JSON object（`{}`） | 400 |
| 模板缺失 / 注入失败 | 500 |
| Gotenberg 不可达 | 502 |
| Gotenberg 转换失败（非超时） | 500 |
| 转换超时（默认 60s） | 504 |

## 模板注入

模板现有模式：

```html
window.__REPORT_DATA__ = { /* 示例数据 */ };
```

服务行为：

1. 从 `template/v1-home-inspection.html` 读取完整 HTML
2. 用 `json.Marshal` 将请求 body（必须是 JSON object）序列化，**只替换** `window.__REPORT_DATA__ =` 右侧的对象字面量 `{...}`，**保留** `window.__REPORT_DATA__ = ...;` 赋值语句本身
3. 将结果作为 `index.html` 以 multipart 提交到 Gotenberg：`POST /forms/chromium/convert/html`
4. 使用固定等待：`waitDelay=0.5`（秒），确保模板内填充 DOM 的脚本执行后再出 PDF

不在服务端用 Go template 引擎渲染 DOM；保留现有客户端 JS 填充方式，由 Chromium 执行。

## 组件边界

| 包/文件 | 职责 | 依赖 |
|---|---|---|
| `main.go` | 注册路由、读配置（Gotenberg URL、超时、模板路径）、启动 HTTP | service |
| `service/pdf_handler.go` | 解析请求、协调注入与转换、写响应 | template + gotenberg client |
| `service/template.go` | 读模板文件、注入 `__REPORT_DATA__` | 文件系统 |
| `gotenberg/client.go` | HTTP 客户端调用本机 Gotenberg convert/html | net/http |

## 容器与构建

1. **builder stage**：`golang` 镜像编译 `main`
2. **runtime stage**：`FROM gotenberg/gotenberg:8`，拷入二进制、`template/`、`docker-entrypoint.sh`
3. entrypoint：启动 Gotenberg（仅监听 localhost:3000）→ `exec` Go（:80）
4. `EXPOSE 80`

删除：`db/`、计数器 service、演示用 `index.html`、MySQL / `container.config.json` 中与库相关的初始化（若保留云托管配置文件，则去掉 DB 相关字段或改为无 DB 的最小配置）。

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `80` | Go 监听端口 |
| `GOTENBERG_URL` | `http://127.0.0.1:3000` | 本机 Gotenberg |
| `TEMPLATE_PATH` | `template/v1-home-inspection.html` | 模板路径 |
| `PDF_TIMEOUT_SEC` | `60` | 转换超时（秒） |

## 测试与验收

本地：

```bash
docker build -t pdf-go-server .
docker run --rm -p 8080:80 pdf-go-server
curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d '{"name":"John Doe","email":"john.doe@example.com","phone":"1234567890","address":"123 Main St","city":"Anytown","state":"CA","zip":"12345","country":"USA"}' \
  -o out.pdf
```

验收标准：

- 容器单端口可访问
- 合法 JSON object 返回可打开的 PDF，内容含注入字段
- 非法 JSON 或非 object 根值返回 400 JSON 错误体
- Gotenberg 未就绪时：`GET /health` 返回 503；`POST /api/pdf` 返回 502

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| 镜像体积大（含 Chromium） | 接受；以官方 Gotenberg 镜像为 runtime 基底 |
| JS 未执行完就截 PDF | 使用 waitDelay / waitForExpression |
| JSON 注入破坏 HTML/JS | 用 `json.Marshal` 结果只替换赋值右侧 `{...}`，保留 `window.__REPORT_DATA__ =` |
| 双进程启动竞态 | entrypoint 等待 Gotenberg health 就绪后再起 Go；`/health` 与 PDF 路径对不可达返回明确错误 |

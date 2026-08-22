# dedao-gui 1.0.3 播放与运行时收口设计

- 日期：2026-08-22
- 状态：用户授权完整执行（不阻塞人工确认）
- 基线：`main@fb4a5f1`（含未推送的 V2 播放修复）
- 目标版本：`1.0.3`
- 目标平台：macOS（含 27.0）、Windows、Linux

## 1. 背景

v1.0.2 完成了凭据、Range 续传、电子书并发和三平台发布门。随后本机提交 `fb4a5f1` 修复了火山 V2 签名播放，但：

1. 该提交未推送，GitHub 发布包仍会把 V2 token 当 `PlayAuthToken` 使用。
2. macOS 窗口仍是透明 WebView（`WebviewIsTransparent` + `WindowIsTranslucent`），在 macOS 27 上会出现进程在前、画面像“透过去”的情况。
3. 第二次启动只打印日志，不把已有窗口拉到前台。
4. 签名查询仍经 Resty `Get("https://vod.volcengineapi.com/?"+query)`，可能被二次编码。
5. 听书下载在 `data == nil` 时会空指针（上游同类闪退）。
6. Wails 钉在 `v2.10.1`，稳定线已到 `v2.14.0`；`v2.12.0` 修了 Tahoe WebView 快刷崩溃。
7. `golang.org/x/crypto` 停在 `0.36.0`。
8. 课程/听书/电子书长列表一次性渲染全部卡片。
9. 下载结束没有系统级通知，最小化后看不到结果。

## 2. 目标

- 本机与发布包使用同一套 V2 播放路径：签名查询原样转发，私有 DRM 运行时参数不进入 `X-SignedQueries`。
- macOS 窗口不透明、可聚焦；第二实例把已有窗口带到前台。
- 桌面运行时升级到 Wails `v2.14.0`，CLI 与 `go.mod` 同源。
- 听书/课程/电子书下载入口拒绝空参数，不再 panic。
- 长列表使用 `content-visibility` 降低离屏渲染成本，不引入新运行时依赖。
- 下载完成/失败/取消发出应用内通知；在已授权时发出系统通知。
- 依赖 `golang.org/x/crypto` 升到 `v0.45.0` 或当前兼容的安全版本。
- 以 `v1.0.3` 发布：tag、`BuildVersion`、`productVersion`、资源名、SHA-256 一致。

## 3. 非目标

- 不重做首页、导航或视觉语言。
- 不合入上游无边框自定义标题栏。
- 不迁移 Wails v3。
- 不新增学习圈下载，不改变内容权限边界。
- 不把 VePlayer 升级为唯一 CDN 版本；保留 1.15.1 → 本地 1.3.5 回退。
- 不在测试或提交中写入真实账号、Cookie、播放 token。
- 不要求本代理在用户桌面上点播验证（用户明确要求不抢鼠标）。端到端出画仍标记为人工验收项。

## 4. 架构

保持现有四层：界面 / 会话 / 下载 / 落盘。本期只改三个边界：

1. **窗口策略**：`backend` 提供可测试的 macOS 窗口策略和“第二实例聚焦”函数；`main.go` 只消费该策略。
2. **火山 HTTP**：`reqVolcQuery` 改为 `net/http`，`url.URL.RawQuery` 使用调用方已校验的查询字节，不再经 Resty 解析。Cookie 与 `User-Agent`/`Xi-DT` 从现有 service 客户端复制。
3. **下载入口**：`OdobDownload`/`CourseDownload`/`EbookDownload` 在调用业务下载前校验必填字段；`extOdobDownloadData` 对 nil 返回错误而不是解引用。

通知走现有下载事件：`DownloadDialog` 在终态触发 `ElNotification`，并在 `Notification` 可用且已授权时发系统通知。不新增 Go 通知依赖。

列表性能用 CSS `content-visibility: auto` 与 `contain-intrinsic-size`，作用于课程/听书/电子书卡片。不引入虚拟列表库。

## 5. 错误处理

- 签名查询含换行或无法解析时返回明确错误，不发请求。
- 火山 HTTP 非 2xx 按现有 `handleHTTPResponse` 语义映射（400/401/404/496）。
- 听书 `data` 为空返回 `下载任务无效`，与 `runDownload(nil)` 同一类错误，不 panic。
- 系统通知被拒绝或不可用时静默回退到应用内通知，不影响下载结果。

## 6. 测试

- 窗口策略：断言透明标志为 false。
- 火山查询：`httptest.Server` 断言收到的 `RawQuery` 与输入完全一致，包括 `X-Signature` 顺序。
- 听书 nil：`OdobDownload{Data: nil}` 返回错误且不 panic。
- 前端：卡片样式包含 `content-visibility`（通过源码契约或现有构建）；下载终态通知函数在未授权时不抛错。
- 回归：`go test ./...`、`go vet`、`npm --prefix frontend test`、`npm --prefix frontend run build`、`scripts/secret-check.sh`。
- Wails 升级后 `go.mod` 中 `github.com/wailsapp/wails/v2` 为 `v2.14.0`，`.wails-version` 同行。

## 7. 发布

下一版本必须是 `1.0.3`。本地 `wails.json.info.productVersion` 保持 `0.0.0`，由 `scripts/set-build-version.sh` 在发布时写入。Quality 与 Release 工作流沿用现有门禁。

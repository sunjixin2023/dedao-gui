# 得到课程下载桌面端

> wails + go + vue 构建的《得到》APP 课程下载桌面客户端

## 技术栈

* [Wails](https://wails.io/zh-Hans/) - 用于构建桌面应用程序
* [Go](https://go.dev/) - 后端服务和业务逻辑
* [Vue 3](https://cn.vuejs.org/guide/introduction.html) - 前端框架
* [Vue Router 4](https://router.vuejs.org/zh/introduction.html) - 路由管理
* [Element Plus](https://element-plus.org/zh-CN/) - UI 组件库
* [TypeScript](https://www.typescriptlang.org/zh/docs/) - 类型安全
* [Vite](https://cn.vitejs.dev/) - 构建工具
* [Pinia](https://pinia.vuejs.org/zh/) - 状态管理
* [FFmpeg](https://ffmpeg.org/) - 音频处理
* [wkhtmltopdf](https://wkhtmltopdf.org/downloads.html) - PDF 生成

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/yann0917/dedao-gui)
[![Go Report Card](https://goreportcard.com/badge/github.com/yann0917/dedao-gui)](https://goreportcard.com/report/github.com/yann0917/dedao-gui)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/yann0917/dedao-gui)

## 特别声明

仅供个人学习使用，请尊重版权，内容版权均为得到所有，请勿传播内容！！！

仅供个人学习使用，请尊重版权，内容版权均为得到所有，请勿传播内容！！！

仅供个人学习使用，请尊重版权，内容版权均为得到所有，请勿传播内容！！！

## 主要功能

* **首页展示** - 展示首页内容概览
* **扫码登录** - 支持二维码扫描登录
* **课程管理** - 可查看**购买**的课程，课程详情，课程文章列表，支持播放课程音频
* **听书功能** - 可查看听书书架列表，听书文稿，支持播放每天听本书音频
* **电子书管理** - 可查看电子书架列表，电子书详情，书评，支持加入书架
* **锦囊查看** - 可查看已购买的锦囊
* **知识城邦** - 可查看知识城邦内容
* **内容导出** - 课程可生成PDF，文稿生成 Markdown 文档，也可生成 mp3 文件
* **视频课下载** - 支持将视频课导出为 MP4 文件（可下载条目）
* **听书下载** - 每天听本书可下载音频，文稿生成 pdf、 Markdown 文档
* **电子书下载** - 电子书可下载 pdf，html, epub 等格式
* **免费内容** - 免费专区的课程如：《每天听本书》，《文明》，《长谈》等，可下载音频，文稿生成 pdf、 Markdown 文档
* **学习圈** - 可查看学习圈内容（暂不支持下载）
* **主题切换** - UI亮色/暗色主题切换

### 注

1. 下载均在后台执行，下载完毕弹框会关闭，等待弹窗关闭或者点击确定下载后关闭，均会在后台执行下载程序。
2. 如果遇到 `496 NoCertificate` 消息提示，请登录网页版进行图形验证码验证。
3. 本应用上登录后再登录官方网页版会导致保存的 cookie 失效，使用 `rm -rf ~/.config/dedao/config.json` 删除配置信息后重新登陆本应用即可。或者进入个人中心，点击退出登录。

### Windows 使用提示

1. 首次运行请在项目目录内执行：`wails dev` 或 `wails build`，不要在 `C:\Users\<用户名>` 直接执行。
2. 如果扫码登录一直无响应，先在网页版重新登录一次，然后重置本地登录缓存：
   * PowerShell: `Remove-Item "$env:USERPROFILE\\.config\\dedao\\config.json" -Force -ErrorAction SilentlyContinue`
3. 若电子书页面没有显示已登录状态，先退出桌面端并重新打开，确保 cookie 已刷新。

### 电子书笔记同步说明

1. 电子书划线/笔记支持调用官方接口进行读取、创建、删除同步。
2. 网络异常或接口失败时，笔记会本地兜底保存并标记为“仅本地”。
3. 恢复网络后重新打开电子书页面，应用会重新拉取并合并官方笔记数据。

## 安装与运行

### 环境要求

1. 安装 Go 1.23 或更高版本
2. 安装 Node.js 18+ 和 npm
3. 安装 Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### 构建步骤

1. 克隆项目仓库
   ```bash
   git clone https://github.com/yann0917/dedao-gui.git
   cd dedao-gui
   ```
2. 直接构建应用（Wails 会自动处理前端依赖安装和构建）
   ```bash
   wails build
   ```

### 一键构建（推荐）

项目已提供统一的一键脚本，默认自动识别当前平台并打包归档：

```bash
make bootstrap
make build
```

也可以指定目标平台构建：

```bash
make release PLATFORM=darwin/universal
make release PLATFORM=windows/amd64
make release PLATFORM=linux/amd64
```

可选参数（传给 `scripts/release.sh`）：

```bash
make build RELEASE_ARGS="--skip-install"
make release PLATFORM=darwin/arm64 RELEASE_ARGS="--no-package"
```

构建产物目录：`build/bin`  
归档产物目录：`release/`（自动生成 `.tar.gz` / `.zip` 和 `.sha256`）

### 一键部署（GitHub Release）

仓库已支持通过 GitHub Actions 自动构建并发布多平台产物，可使用一条命令触发：

```bash
make deploy VERSION=v1.0.0
```

等价于手动创建并推送 tag：

触发方式：

```bash
git tag v1.0.0
git push origin v1.0.0
```

触发后会自动构建：

* macOS Universal
* Windows amd64
* Linux amd64

并在 GitHub Release 中上传对应压缩包。

详细构建说明请参考 [Wails 文档](https://wails.io/zh-Hans/docs/introduction)

### 必需依赖

项目运行需要以下依赖：

* **Go** 1.23+ - 后端开发语言
* **Node.js** 18+ - 前端运行环境
* **npm** - 前端包管理器

### 可选依赖（根据需求安装）

如需使用特定功能，请安装以下依赖：

#### PDF 生成

* **wkhtmltopdf**
  > 电子书转 PDF 需要借助 [wkhtmltopdf](https://wkhtmltopdf.org/downloads.html)

#### 音频处理

* **ffmpeg**
  > 音频合成及处理需要借助 [ffmpeg](https://ffmpeg.org/) 工具

## 功能预览

![](image/Snipaste_2026-01-21_15-22-11.jpg)
![](image/Snipaste_2026-01-21_15-22-47.jpg)
![](image/Snipaste_2026-01-21_15-23-09.jpg)
![](image/Snipaste_2026-01-21_15-23-22.jpg)
![](image/Snipaste_2026-01-21_15-23-33.jpg)
![](image/Snipaste_2026-01-21_15-31-02.jpg)
![](image/Snipaste_2026-01-21_15-33-17.jpg)
![](image/Snipaste_2026-01-21_15-23-47.jpg)
![](image/Snipaste_2026-01-21_15-24-28.jpg)
![](image/Snipaste_2026-01-21_15-25-23.jpg)
![](image/Snipaste_2026-01-21_15-26-03.jpg)

## Stargazers over time

[![Stargazers over time](https://starchart.cc/yann0917/dedao-gui.svg)](https://starchart.cc/yann0917/dedao-gui)

## License

[MIT](./LICENSE) © yann0917

---

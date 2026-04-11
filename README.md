# CZL Browser

> 面向多账号隔离、代理绑定和本地环境管理的 Windows 桌面浏览器工具。

[![Release](https://img.shields.io/github/v/release/woodchen-ink/CZL-Browser?sort=semver)](https://github.com/woodchen-ink/CZL-Browser/releases)
[![Platform](https://img.shields.io/badge/Platform-Windows%2010%2F11-blue)](https://github.com/woodchen-ink/CZL-Browser/releases)
[![Issues](https://img.shields.io/github/issues/woodchen-ink/CZL-Browser)](https://github.com/woodchen-ink/CZL-Browser/issues)

CZL Browser 的目标很明确：在一台 Windows 设备上，帮助用户稳定管理多个彼此隔离的浏览器实例，并配合代理池、浏览器内核和快捷启动能力完成日常运营或测试工作。

## 目录

- [项目简介](#项目简介)
- [核心特性](#核心特性)
- [界面预览](#界面预览)
- [快速开始](#快速开始)
- [常用操作](#常用操作)
- [常见问题](#常见问题)
- [Roadmap](#roadmap)
- [贡献](#贡献)
- [支持与反馈](#支持与反馈)
- [License](#license)

## 项目简介

CZL Browser 适合以下场景：

- 多账号环境隔离
- 跨境电商与社媒账号运营
- 需要独立代理出口的本地测试
- 需要统一管理浏览器内核和实例配置的团队

这个项目当前提供的核心价值是：

- 给每个账号分配独立浏览器实例
- 给每个实例绑定独立代理
- 统一管理浏览器内核、标签和关键字
- 在本地保存配置和运行数据，便于自主控制

## 源码分支说明

- `master`：面向开发者的干净基线分支，不提交 `data/app.db`、实例目录或其他用户数据。首次启动时会自动初始化空数据库。
- `user_data`：在 `master` 基础上额外提交一份 `data/app.db` 测试快照，便于演示、联调和复现问题。
- 代理运行时 `bin/xray.exe`、`bin/sing-box.exe` 已随源码仓库提供；开发和发布打包不需要再单独下载这些运行时文件。

## 核心特性

- 实例隔离管理：支持创建、编辑、启动、停止、重启、克隆和删除浏览器实例
- 代理池配置：支持统一维护代理节点，并将代理分配到具体实例
- 多协议支持：支持常见代理配置方式，并支持导入 Clash
- 内核管理：支持维护多个 Chrome 内核版本，并设置默认内核
- 快捷启动：支持通过 `Ctrl + K` 快速检索并打开目标实例
- 标签与检索：支持按标签、关键字、状态、代理、内核、分组进行筛选
- 本地化存储：配置和实例数据保存在本地，适合长期使用和备份

## 界面预览

### 1. 控制台

<img src="images/readme/001-首页.png" alt="控制台" width="100%" />

对应功能点：

- 查看实例总数、运行中实例、代理节点数量和内核版本
- 从首页快速进入 `实例列表`、`代理池配置`、`内核管理`、`系统设置`
- 查看客户端版本、运行状态和资源占用概览

### 2. 实例列表

<img src="images/readme/002-实例列表.png" alt="实例列表" width="100%" />

对应功能点：

- 统一查看和管理所有浏览器实例
- 按状态、代理、内核、分组、关键字筛选实例
- 支持 `新建配置`、启动、停止、重启、配置、克隆、删除
### 3. 代理池配置

<img src="images/readme/003-设置代理池.png" alt="代理池配置" width="100%" />

对应功能点：

- 统一管理代理节点
- 支持按协议、分组筛选代理
- 支持手动维护代理和导入 Clash
- 支持查看延迟、IP 健康并挑选可用节点

### 4. 代理生效验证

<img src="images/readme/004-自定义代理.png" alt="代理生效验证" width="100%" />

对应功能点：

- 启动实例后访问 IP 检测网站验证代理是否真正生效
- 检查 IP 地区、ASN、运营商和风险值等信息
- 用于确认当前实例是否已经走目标代理出口

## 快速开始

### 环境要求

- 操作系统：Windows 10 / Windows 11（64 位）
- 建议内存：8 GB 及以上
- 建议磁盘空间：2 GB 以上

### 下载与运行

1. 前往 Releases 页面下载最新版本：<https://github.com/woodchen-ink/CZL-Browser/releases>
2. 安装版直接运行 `CZLBrowser-Setup-*.exe`
3. 便携版解压后运行 `czl-browser.exe`

### 从源码运行

1. 开发默认使用 `master` 分支；该分支不带测试用户数据，适合作为日常开发基线。
2. 如需带测试库的演示环境，请切换到 `user_data` 分支。
3. 执行 `bat\dev.bat` 或直接使用 `wails dev` 启动项目。
4. 仓库已内置 `bin/xray.exe` 和 `bin/sing-box.exe`，不需要额外下载代理运行时。

### 准备浏览器内核

代理运行时已经随仓库提供，你只需要准备浏览器内核。

1. 打开应用，进入 `指纹浏览器 > 内核管理`
2. 使用应用内下载功能准备内核
3. 下载完成后，在内核管理页面确认状态正常并设置默认内核

### 第一次使用建议流程

1. 在 `代理池配置` 中先导入或新增可用代理节点
2. 在 `实例列表` 中点击 `新建配置`
3. 选择实例名称、内核、代理、标签和需要的启动参数
4. 返回实例列表，点击启动按钮运行实例
5. 打开 IP 检测网站，确认代理结果是否符合预期

## 常用操作

| 目标 | 入口 | 说明 |
| --- | --- | --- |
| 新建浏览器实例 | `实例列表 > 新建配置` | 创建一个新的独立浏览器环境 |
| 配置代理池 | `代理池配置` | 维护代理节点并检查延迟、健康状态 |
| 绑定实例代理 | `实例编辑页` | 给指定实例分配目标代理节点 |
| 启动实例 | `实例列表` | 单击启动按钮即可运行目标实例 |
| 快速打开实例 | `Ctrl + K` | 可按实例名、标签、关键字快速检索 |
| 管理浏览器内核 | `内核管理` | 下载内核、删除内核和设置默认内核 |
| 验证代理结果 | 启动实例后访问 IP 检测网站 | 核对 IP、地区、ASN、风险值 |

## 常见问题

### 1. 应用无法启动怎么办？

先检查浏览器内核路径是否有效，并确认目标目录下存在 `chrome.exe`。

### 2. 实例启动了但代理没有生效怎么办？

先检查代理节点本身是否可用，再确认该实例已经正确绑定代理。建议启动后访问 IP 检测网站复核当前出口。

### 3. 实例太多，怎么快速找到目标实例？

可以在 `实例列表` 中按状态、代理、内核、分组、关键字筛选，也可以通过 `Ctrl + K` 直接搜索目标实例并启动。

### 4. 多个账号怎么避免串号？

建议采用一账号一实例、一实例一稳定代理的方式，不要混用浏览器环境，也不要频繁切换同一实例的出口 IP。

## Roadmap

- 增强实例模板、批量管理和检索体验
- 优化公开版本的首次使用引导与基础说明

## 商业化实施文档

商业化、企业版和在线更新的分阶段实施文档已整理到：

- `docs/commercialization/README.md`
- `docs/commercialization/00-overview.md`
- `docs/commercialization/01-release-and-updates.md`
- `docs/commercialization/02-browser-control-plane.md`
- `docs/commercialization/03-client-foundation.md`
- `docs/commercialization/04-billing-and-proxy-offers.md`
- `docs/commercialization/05-enterprise-workbench.md`
- `docs/commercialization/06-ops-backoffice-and-analytics.md`

这些文档定义了 `CZL Connect + Browser Control Plane + 单客户端双模式` 的职责边界、发版更新链路、企业工作台和商业化落地顺序，后续相关改动以这些文档为准同步推进。

## 贡献

欢迎通过 Issue 和 Pull Request 参与改进。

- Bug 反馈：请附带版本号、系统版本、复现步骤和截图
- 功能建议：请说明业务场景、预期行为和现有问题
- 文档优化：欢迎直接提交 README、教程和截图说明相关改进

如果是较大改动，建议先开 Issue 对齐需求再提交 PR。

## 支持与反馈

- Releases：<https://github.com/woodchen-ink/CZL-Browser/releases>
- Issues：<https://github.com/woodchen-ink/CZL-Browser/issues>

## License

当前仓库暂未附带独立的 `LICENSE` 文件，后续会补充。

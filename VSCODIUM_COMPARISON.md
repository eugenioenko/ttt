# TTT 与 VSCodium 功能对比表

> 对比日期：2026-11-03。这里的 VSCodium 指基于 VS Code 开源代码构建、默认关闭 Microsoft telemetry、默认使用 Open VSX 的发行版；VSCodium 的大量 IDE 能力来自 VS Code 内建能力与扩展生态，而不是 VSCodium 独立开发。

## 1. 结论摘要

TTT 的优势是终端原生、低依赖、键盘/脚本优先、Lua 插件可控权限、深度 Git diff/PR 审查和内置 CLI Agent 会话。VSCodium 的优势是完整桌面 Workbench、成熟的 VS Code 扩展 API/生态、调试器、测试资源管理器、任务/运行配置、远程开发、Profiles/Settings Sync 和无障碍能力。两者定位不是简单的“谁替代谁”：TTT 适合 terminal-first workflow；VSCodium 适合 desktop IDE workflow。

## 2. 功能对比

标记：✅ 已具备；◐ 部分具备/依赖外部工具或插件；— 未发现同等内建能力；△ 可通过扩展或脚本补齐。

| 领域 | 功能 | TTT | VSCodium | 差异与判断 |
|---|---|:---:|:---:|---|
| 运行形态 | 跨平台代码编辑器 | ✅ | ✅ | TTT 为终端程序；VSCodium 为 Electron 桌面 Workbench |
| 安装/分发 | 单二进制、源码构建、包管理 | ✅ | ✅ | VSCodium 提供桌面安装包与每日构建 |
| 遥测 | 默认不依赖 Microsoft telemetry | ✅ | ✅ | VSCodium 明确以 telemetry-free 为目标 |
| 编辑 | 基础编辑、Unicode、剪贴板、撤销重做 | ✅ | ✅ | 两者均完整；TTT 强调 rune/终端列宽正确性 |
| 导航 | 行号跳转、Quick Open、命令面板 | ✅ | ✅ | VSCodium 还拥有更完整的 Workbench 导航和符号 UI |
| 多光标 | 匹配选择、全选匹配、鼠标加光标 | ✅ | ✅ | 能力接近 |
| 折叠 | 缩进折叠与折叠导航 | ✅ | ✅ | TTT 另有 LSP foldingRange 待实现；VSCodium 可由语言扩展提供 |
| 高亮 | TextMate 语法高亮 | ✅ 124 语言 | ✅ | VSCodium 依赖内建语言支持/扩展，生态覆盖更广 |
| IntelliSense | 补全、签名、悬停、自动导入 | ◐ LSP | ✅ | VSCodium 的语言扩展数量和语言专用体验更广 |
| 代码导航 | 定义、实现、类型定义、引用、重命名 | ✅ LSP | ✅ | 两者均依赖语言服务；TTT 已覆盖主要 LSP 方法 |
| 代码操作 | 格式化、范围格式化、code action | ✅ | ✅ | TTT 支持 LSP/外部 formatter；VSCodium 通过扩展更丰富 |
| 诊断 | inline squiggle、Problems、状态计数 | ✅ | ✅ | VSCodium 还有更成熟的 Problems 工作流 |
| Outline/符号 | 文档符号、工作区符号 | — LSP 待实现 | ✅ | TTT 明确列为中影响未实现项 |
| Inlay/CodeLens | 内联提示、CodeLens、selection range | — | ✅/◐ | VSCodium 依赖语言扩展；TTT LSP 未实现 |
| 搜索 | 文件内查找替换 | ✅ | ✅ | 能力接近 |
| 全局搜索 | ripgrep、多根、glob 过滤 | ✅ | ✅ | VSCodium 有更完整的搜索 UI/排除规则生态 |
| 文件树 | 多根 Explorer、创建/重命名/删除 | ✅ | ✅ | VSCodium 有更多视图、Timeline、装饰扩展 |
| 外部变化 | watcher 与打开文件变更处理 | ✅ | ✅ | 能力接近 |
| 工作区 | 多根、保存 `.ttt` 工作区 | ✅ | ✅ `.code-workspace` | 格式不同，VSCodium 可存布局/任务/调试配置更多 |
| Git 状态 | status、stage、unstage、commit | ✅ | ✅ | 能力接近 |
| Git 历史 | 提交详情、分页历史、blame | ✅ | ✅/△ | VSCodium 的图谱/Timeline 常由内建或扩展增强 |
| Git diff | 可编辑 diff、changes-only、差异随输入更新 | ✅ | ✅ | TTT 在终端 diff 审查与编辑联动上有鲜明优势 |
| 分支/远程 | branch、pull、push、sync | ✅ | ✅ | 能力接近 |
| 冲突解决 | 基础 Git 冲突能力 | ◐ | ✅ 三方合并编辑器 | TTT 后续应补齐 merge editor 与冲突动作 |
| PR 审查 | GitHub URL、提交/变更审查 | ✅ `gh` | ◐/✅ | VSCodium 通常依赖 GitHub Pull Requests 扩展 |
| 集成终端 | PTY、多标签、VT、scrollback | ✅ | ✅ | TTT 的终端是核心场景；VSCodium 提供 split/move/profile 等桌面布局 |
| 终端颜色 | 24-bit、256 色、主题 ANSI | ✅ | ✅ | 能力接近 |
| 终端链接 | URL/文件链接 | ✅ | ✅ | VSCodium 还有 shell integration 与扩展 link handlers |
| 输出日志 | LSP、插件、通知统一 Output panel | ✅ | ✅ | VSCodium 还有 Debug Console、Problems、Tasks 等专用通道 |
| 调试 | 断点、单步、变量、调用栈、DAP | — | ✅/◐ | TTT 当前没有 DAP/debugger 产品面 |
| 测试 | Test Explorer、运行/调试、覆盖率 | — | ✅/◐ | VSCodium 依赖测试扩展；TTT 仅有自身测试与终端执行 |
| 任务 | tasks.json、build/run task、后台任务 | ◐ 命令/终端 | ✅ | TTT 可通过插件/Agent/终端替代，但无同等任务模型 |
| 语言生态 | LSP 配置与插件 | ✅ | ✅ | VSCodium 还支持大量专用语言扩展、调试器和 snippets |
| 扩展来源 | Lua 社区注册表、Git URL、manifest | ✅ | ✅ Open VSX | VSCodium 默认 Open VSX；部分 Microsoft 扩展受许可限制不可用 |
| 扩展 API | Lua Widget/面板/命令/事件/RPC | ✅ | ✅ VS Code Extension API | 两者 API 不兼容；VSCodium 生态规模明显更大 |
| 安全模型 | 插件权限逐项审批、沙箱 | ✅ | ◐ Workspace Trust/扩展权限 | TTT 权限模型更显式；VSCodium 的扩展 API 更强但权限面更大 |
| UI 定制 | JSON 设置、快捷键、主题、面板 | ✅ | ✅ | VSCodium 还有布局保存、Profiles、产品级设置 UI |
| 主题 | 大量内置主题、全色彩可配、终端 ANSI | ✅ | ✅ | TTT 主题可控且与终端统一；VSCodium 生态更多 |
| Markdown | Markdown styled rendering、表格、代码块 | ✅ | ✅/△ | VSCodium 有预览、编辑器扩展和 WebView 生态 |
| 图片/图形 | Kitty graphics placement、插件 graphics | ✅ | ✅/△ | TTT 面向终端图形协议；VSCodium 以桌面/WebView 为主 |
| AI | 多 CLI Agent 持久会话 | ✅ | ✅/◐ | VSCodium 当前有内建/扩展 Agent 工作流；TTT 将 CLI Agent 作为终端原生面板 |
| 远程开发 | SSH、容器、WSL、Codespaces、Web | — | ✅/◐ | TTT 适合 SSH 内直接运行，但无 VSCodium Remote 扩展体系 |
| 设置同步 | 跨设备 Settings Sync | — | ✅ | TTT 当前使用文件/版本库同步配置 |
| Profiles | 按项目切换设置/扩展/UI | — | ✅ | 可作为 TTT 后续工作区配置能力 |
| 无障碍 | 屏幕阅读器、Accessible View、高对比 | ◐ 终端可访问性依赖宿主 | ✅ | VSCodium 的桌面辅助技术能力更完整 |
| Web 开发 | 内置浏览器/DOM 调试/预览 | — | ◐/✅ | VSCodium 近年能力与扩展生态更强 |
| 可测试性 | SimScreen、功能脚本、PTY、Chaos | ✅ | ◐ | TTT 对自身 TUI 行为有更强的确定性测试；VSCodium 面向桌面自动化 |
| 资源占用 | 终端、单进程、轻量 UI | ✅ | ◐ | 实际占用取决于扩展和语言服务；TTT 目标更轻量 |

## 3. 重点差距与产品建议

### 3.1 TTT 应保持的差异化

- terminal-first：不用离开终端即可完成编辑、Git、LSP、Agent 和 PTY 工作流。
- 可脚本化：`--exec`、`--listen`、debug dump 和确定性测试让问题可复现。
- 安全插件：Lua API 按权限授权，比任意扩展进程模型更容易审计。
- Git 审查：可编辑 diff、changes-only、PR review 和多根仓库是当前亮点。
- 资源效率：增量高亮、差异渲染、按需服务启动适合大文件和远程环境。

### 3.2 最值得补齐的 VSCodium 差距

1. LSP Outline、Workspace Symbol、document highlight、inlay hint、code lens。
2. DAP 调试器与断点/调用栈/变量 UI。
3. Test Explorer、测试运行/调试/覆盖率。
4. Tasks/launch 配置与项目级运行入口。
5. 三方合并编辑器、Git 图谱、stash/worktree、逐行/逐块暂存。
6. 工作区 Profiles、配置同步、Workspace Trust 和更完整无障碍路径。
7. 更广的语言/插件兼容层，或提供 VS Code Extension API 的可行子集。

## 4. 对比口径与来源

- [VSCodium README](https://github.com/VSCodium/vscodium/blob/master/README.md)：VSCodium 构建方式、遥测定位和 Open VSX 说明。
- [VSCodium Extensions + Marketplace](https://github.com/VSCodium/vscodium/blob/master/docs/extensions.md)：默认 Open VSX、扩展兼容性和 Microsoft Marketplace 限制。
- [VS Code Core Editor Overview](https://code.visualstudio.com/docs/editing/getting-started/overview)：编辑、工作区、终端、SCM、调试、测试、扩展和 Profiles 总览。
- [VS Code Source Control](https://code.visualstudio.com/docs/sourcecontrol/overview)：Git、diff、stage/commit、分支、worktree、stash、冲突和历史能力。
- [VS Code Testing](https://code.visualstudio.com/docs/debugtest/testing)：Test Explorer、测试运行/调试、结果和覆盖率。
- [VS Code Programming Languages](https://code.visualstudio.com/docs/languages/overview)：语言扩展提供的高亮、补全、诊断、导航、调试和重构边界。
- [VS Code Terminal Basics](https://code.visualstudio.com/docs/terminal/basics)：集成终端、shell、链接和工作区上下文。

## 5. 结论

若目标是“终端内完成高频开发闭环”，TTT 已具备清晰且有竞争力的产品基础；若目标是“完全替代 VSCodium 的桌面 IDE”，最大缺口不在基础编辑，而在 DAP、测试、任务、远程开发、Profiles/Sync、无障碍和 VS Code 扩展兼容生态。建议产品路线优先巩固 TTT 的终端差异化，再按用户需求补齐 LSP 语义视图、调试、测试和项目运行模型。

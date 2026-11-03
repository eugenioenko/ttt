# TTT（Terminal Text Tool）功能需求清单 PRD

> 文档版本：v1.0 ｜ 盘点日期：2026-11-03 ｜ 依据：当前仓库代码、README、ARCHITECTURE.md、docs-web、测试与配置文件。

## 1. 产品定义

TTT 是一个 Go 编写、以终端为主要交互界面的现代代码编辑器。产品目标是在低资源、键盘优先、可脚本化的环境中，提供接近现代 IDE 的编辑、代码智能、Git、终端、插件与 AI Agent 工作流，同时保持单一二进制、可测试和可扩展。

### 1.1 目标用户

- 偏好终端和键盘操作的开发者。
- 需要在 SSH、容器、低带宽或无桌面环境中工作的开发者。
- 使用 Git、多文件夹项目、LSP 和命令行 Agent 的工程师。
- 希望通过 Lua 插件定制工作流、面板和自动化能力的高级用户。

### 1.2 产品原则

1. 编辑体验优先：光标、选区、撤销、宽字符、折叠和滚动必须稳定。
2. 命令与 UI 双入口：核心能力可从命令面板、快捷键和脚本调用。
3. 终端原生：全功能 PTY、VT、真彩色和终端聚焦行为不可破坏。
4. 数据安全：文件、Git、插件和外部命令有清晰边界；插件采用权限模型。
5. 可观测、可复现：异步事件回到主线程，支持 `--exec`、debug dump 和确定性测试。

## 2. 功能需求总览

优先级：P0=核心可用性，P1=完整开发工作流，P2=增强与生态，P3=未来规划。

| 模块 | 需求范围 | 当前状态 | 优先级 |
|---|---|---:|---:|
| 启动与工作区 | CLI、文件/目录、多根工作区、`.ttt` 工作区、PR URL、stdin | 已实现 | P0 |
| 编辑器 | 文本编辑、选择、撤销、导航、折叠、多光标、宽字符 | 已实现 | P0 |
| 代码理解 | TextMate 高亮、LSP 补全/导航/诊断/格式化/重命名 | 已实现（部分 LSP 方法未覆盖） | P0 |
| 文件与资源管理 | Explorer、文件操作、图标、外部变更监控 | 已实现 | P0 |
| 搜索 | 文件内查找替换、跨工作区 ripgrep 搜索、过滤 | 已实现 | P0 |
| Git | 状态、暂存、提交、分支、远程、diff、blame、PR 审查 | 已实现 | P1 |
| 终端 | 多标签 PTY、VT、滚动、链接、真彩色、终端焦点 | 已实现 | P1 |
| UI 与配置 | 面板、菜单、主题、设置表单、快捷键、Markdown | 已实现 | P1 |
| 插件平台 | Lua、注册表、权限、面板/命令/Widget/API/RPC | 已实现 | P1/P2 |
| AI Agent | 右侧持久会话、多供应商配置、安装命令 | 已实现 | P1 |
| 可测试性 | 单元、E2E、功能、集成、Chaos、benchmark、debug harness | 已实现 | P0 |
| IDE 扩展能力 | 调试器、测试资源管理器、任务系统、远程开发、Profile/Sync | 未发现完整内建实现 | P2/P3 |

## 3. 详细功能需求

### FR-01 启动、文件与工作区

- 支持打开当前目录、单/多个目录、具体文件、`file:line:column`、stdin 和 `.ttt` 工作区。
- 支持从 GitHub Pull Request URL 进入审查工作区。
- 多根工作区按根目录分组；Explorer、Search、Changes 使用同一工作区上下文。
- 当前文件按最长路径匹配所属 Folder；状态栏 Git 分支随当前文件切换。
- 提供 `--workspace`、`--exec`、`--listen`、`--size`、`--plugin` 等脚本/调试入口。

### FR-02 编辑器核心

- 行式文本缓冲区，rune index 光标，支持 Unicode 和全角字符。
- 插入、删除、换行、合并/拆分行、复制/剪切/粘贴、系统/OSC52/进程剪贴板。
- 撤销/重做必须通过 EditCommand；批量编辑合并为一个撤销步骤。
- 查找、替换、大小写转换、行移动/复制/删除、排序、注释切换。
- 多光标：选择下一个匹配、全选匹配、Alt+Click 添加光标；编辑操作同步应用。
- 折叠：缩进折叠、折叠/展开全部、折叠感知导航和搜索。
- 自动换行、缩进检测、`.editorconfig`、Tab/Space 设置、保存前格式化。
- 鼠标定位、标签点击、右键菜单、拖动面板分隔线。
- 编辑器渲染使用双缓冲差异渲染；所有布局宽度使用统一的 display width 规则。

### FR-03 语法与语言服务

- 内置 TextMate 高亮，支持 124 个语法，支持多行结构和增量高亮。
- 支持 LSP 生命周期、全量文档同步、诊断、补全、补全解析、签名帮助、悬停。
- 支持跳转定义/实现/类型定义、引用、符号重命名、文档/选区格式化、代码操作。
- 支持 LSP 服务状态、服务日志、失败提示、重启和配置化启动。
- 当前未实现/待补齐：文档符号、工作区符号、文档高亮、LSP foldingRange、selectionRange、codeLens、inlayHint、linkedEditingRange。

### FR-04 Explorer、搜索与变更监控

- Explorer 支持目录树、多根根节点、排序、展开/折叠、刷新、创建、重命名、删除。
- 可选 Nerd Font 文件图标；支持自动 reveal 当前文件。
- ripgrep 跨工作区搜索，智能大小写、include/exclude glob、结果分组和点击跳转。
- 文件系统 watcher 发现外部变更；对打开文件提供重新加载/冲突处理路径。

### FR-05 Git 与代码审查

- 发现仓库、读取状态、处理新增/修改/删除/重命名/未跟踪文件。
- 文件和批量 stage/unstage/discard；提交消息输入与提交。
- pull、push、sync；分支创建/切换；支持嵌套仓库与 worktree 场景。
- changes-only、完整上下文、统一/分栏 diff；可编辑工作树 diff，差异随输入更新。
- Git gutter、当前行 blame、提交历史分页、提交详情 Markdown 和文件 diff。
- GitHub PR review：按提交查看、变更视图、远程 PR 拉取与评审。
- 冲突处理和更丰富的历史/图谱能力是后续增强项。

### FR-06 终端与输出

- 集成 PTY 终端，支持多终端标签、VT 解析、滚动、shell/scrollback 配置。
- 支持 24-bit/256 色，终端 ANSI 色彩来自主题；终端聚焦时按 RawKeyConsumer 将按键直通 PTY。
- 支持终端链接识别及打开文件/URL；异步 PTY 输出唤醒事件循环。
- Output panel 汇聚插件日志、LSP 日志和通知；后台线程只能通过事件回主线程更新 UI。

### FR-07 UI、主题与配置

- 根 UI 支持编辑区、Sidebar、Bottom Panel、右侧 Agent panel、overlay、dialog、drawer、menu、tabs、table、tree、list、input 等组件。
- 所有颜色通过 ThemeConfig/StyleDef/term.Style/BuildStyleMap 统一管理；终端直 RGB 是特例。
- 支持内置主题、用户主题、实时切换、语义颜色、diff/terminal/syntax 配色。
- 支持 JSON settings、keybindings、表单设置编辑、Apply/Cancel、重启型设置提示。
- 支持快捷键、Chord、命令面板、Quick Open、可搜索帮助和 force key。
- Markdown 支持标题、列表、表格、代码块、链接、引用、换行和样式渲染。

### FR-08 插件与扩展平台

- Lua 插件 manifest、加载/启停/更新/卸载/重载、社区注册表、URL 安装。
- 插件可注册 Sidebar/Bottom/Drawer/Editor panel、命令、快捷键、文件处理器、状态栏项目和 tab。
- Widget API 提供 tree/table/list/input/dialog/dropdown/tabs/scrollview/markdown 等组件。
- 提供编辑器读写、诊断、装饰、剪贴板、终端、文件系统、网络 HTTP/RPC、系统命令、定时器、存储、屏幕读取等 API。
- 权限按能力拆分，并在首次加载或权限变化时请求用户批准；插件系统可整体禁用。

### FR-09 AI Agent

- 右侧面板维护多个持久命令行 Agent 会话；每个供应商拥有独立 PTY。
- 内置 Codex、Claude Code、CodeBuddy、GitHub Copilot CLI、Cursor Agent、Kimi CLI 配置入口。
- 支持可配置 executable、参数和安装命令；缺失时可在内置终端执行安装。
- Agent 面板与普通终端分离，支持 View 菜单和命令面板打开。

### FR-10 质量、性能和运维约束

- 单元测试覆盖核心 buffer/cursor/undo/diff/LSP/JSON-RPC/插件/Markdown/highlight 等纯逻辑。
- E2E 使用 SimScreen 验证 App 组合行为；功能测试驱动真实二进制；集成测试覆盖真实 PTY/LSP/外部变更；Chaos 测试验证稳定性。
- 提供 editor/diff/log/code benchmark；大文件、Unicode、异步事件和终端输出不能阻塞 UI。
- 发布前至少通过 `make test`、`make lint`、受影响包聚焦测试和真实 `--exec` 回归。

## 4. 非功能需求

| 类别 | 要求 |
|---|---|
| 平台 | Go 生态支持的平台；终端具备 Unicode/真彩色/PTY 能力时体验最佳 |
| 性能 | 差异渲染、增量高亮；避免全缓冲重 tokenization；大文件操作可观测 |
| 可靠性 | 异步结果不直接改 widget；事件循环可唤醒、可取消；关闭释放 PTY/LSP/待处理请求 |
| 安全 | 插件权限最小化；系统命令、网络、文件访问显式授权；敏感信息不写日志 |
| 可用性 | 键盘优先，同时支持鼠标；高对比主题；不以固定颜色和固定字符宽度假设布局 |
| 可维护性 | 遵守 ARCHITECTURE.md 依赖分区；核心领域不依赖 tcell；命令使用 `domain.verbNoun` |

## 5. 版本路线建议

- **当前基线**：巩固现有编辑、LSP、Git、终端、插件和 Agent 能力，补齐文档与边界测试。
- **下一阶段 P1**：LSP Outline/Workspace Symbol、冲突三方合并、Git 历史图谱、任务运行入口、问题聚合体验。
- **P2**：DAP 调试器、测试资源管理器、运行配置、代码覆盖率、更多 SCM provider、插件兼容层。
- **P3**：远程/容器开发、配置 Profile/Sync、浏览器预览、跨设备工作区同步。

## 6. 验收基线

每项用户可见功能必须有：需求 ID、命令/快捷键入口、错误/空状态、受影响配置、至少一个最低层确定性测试；涉及启动、外部进程或可见组合行为时，再补充 E2E/功能/集成测试。可见 UI 变更需附 `--exec` 截图或等价可复现脚本。

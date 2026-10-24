---
title: AI Agents / AI Agent
description: Use coding agents inside TTT's right-side terminal panel.
sidebar:
  order: 7
---

# AI Agents / AI Agent

## English

TTT can host several command-line coding agents in persistent sessions on the right side of the editor. The session is backed by TTT's interactive terminal, so an agent keeps its normal terminal UI, working directory, colors, and interactive behavior. Hiding the panel does not terminate the session; selecting the provider again restores it.

### Supported providers

- Codex
- Claude Code
- CodeBuddy
- GitHub Copilot CLI
- Cursor Agent
- Kimi CLI

Open the **View** menu and select an agent, or open the command palette (`Ctrl+P`) and run **View: Use … Agent**. Each provider has its own session. The regular integrated terminal remains separate in the bottom panel.

When an agent is selected for the first time, TTT checks whether its configured executable is available. If it is missing and an install command is configured, TTT runs that command in the embedded terminal and starts the agent after installation succeeds. Installation commands are ordinary shell commands: review them before use and make sure the required package manager is installed.

### Configuration

Agent settings are stored under `agents` in `~/.config/ttt/settings.json`. Each entry supports:

| Field | Meaning |
| --- | --- |
| `cli` | Executable to launch |
| `args` | Additional command-line arguments |
| `install` | Optional shell command used when `cli` is not found |

Example:

```json
{
  "agents": {
    "codex": {
      "cli": "codex",
      "install": "npm install -g @openai/codex"
    },
    "claude": {
      "cli": "claude",
      "args": "-p",
      "install": "npm install -g @anthropic-ai/claude-code"
    }
  }
}
```

The same fields can be edited from **Settings → AI Agents**. Existing configurations using the former top-level `codex` block are migrated automatically when loaded.

## 中文

TTT 现在可以在编辑器右侧托管多个命令行编码 Agent。每个 Agent 都运行在 TTT 的交互式终端会话中，因此可以保留自身的终端界面、工作目录、颜色和交互行为。隐藏右侧面板不会结束会话，再次选择对应 Agent 即可恢复。

### 支持的 Agent

- Codex
- Claude Code
- CodeBuddy
- GitHub Copilot CLI
- Cursor Agent
- Kimi CLI

打开 **View** 菜单选择 Agent，或者按 `Ctrl+P` 打开命令面板，执行 **View: Use … Agent**。不同 Agent 使用独立会话；普通集成终端仍然位于底部面板，不会与 Agent 会话混用。

首次选择某个 Agent 时，TTT 会检查配置的可执行文件是否存在。如果找不到且配置了安装命令，TTT 会在嵌入式终端中执行安装命令，安装成功后再启动 Agent。安装命令就是普通 Shell 命令，请先检查命令内容，并确认系统已安装所需的软件包管理器。

### 配置方式

Agent 配置位于 `~/.config/ttt/settings.json` 的 `agents` 节点下。每个 Agent 支持以下字段：

| 字段 | 含义 |
| --- | --- |
| `cli` | 要启动的可执行文件 |
| `args` | 额外的命令行参数 |
| `install` | 找不到 `cli` 时执行的可选 Shell 安装命令 |

示例：

```json
{
  "agents": {
    "codex": {
      "cli": "codex",
      "install": "npm install -g @openai/codex"
    },
    "claude": {
      "cli": "claude",
      "args": "-p",
      "install": "npm install -g @anthropic-ai/claude-code"
    }
  }
}
```

也可以通过 **Settings → AI Agents** 编辑这些字段。旧版本使用的顶层 `codex` 配置块在读取时会自动迁移到新的 `agents.codex` 配置中。

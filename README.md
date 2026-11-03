# Antigravity TUI

A high-performance Terminal User Interface (TUI) client for **Google Antigravity**, built in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

Designed to run in a terminal on another desktop / workspace in **Hyprland** (or over SSH), connecting directly to the local Antigravity hub socket with zero manual configuration.

---

## ✨ Features

- **Zero-Config Auto-Discovery**: Automatically discovers running Antigravity instances via `~/.gemini/antigravity/daemon/hub_*.json`, reading the HTTP port and CSRF token.
- **WebSocket RPC Protocol**: Speaks the native Connect/gRPC JSON-framed WebSocket protocol directly to `/connect-websocket`.
- **Dual-Pane Layout**:
  - **Left Pane (Sidebar)**: Real-time list of all conversation trajectories, status badges (`RUNNING`, `IDLE`), step counters, and last modified timestamps.
  - **Right Pane (Chat Transcript & Input)**:
    - Formatted chat history with user messages, assistant responses, model reasoning/thoughts (`💭 ...`), and tool execution logs (`🔧 run_command`, etc.).
    - Interactive prompt textarea with multiline support.
- **Agent Control**:
  - Send messages and prompt instructions directly to active agent sessions.
  - Interrupt / cancel running agent turns with `Esc`.
- **Lightweight Single Binary**: Compiles down to a standalone binary with zero external runtime dependencies.

---

## 🚀 Quick Start

### Build
```bash
cd ~/antigravity-tui
go build -o bin/antigravity-tui ./cmd/antigravity-tui
```

### Run
Simply run the binary from any terminal:
```bash
./bin/antigravity-tui
```

### Optional Flags
```bash
./bin/antigravity-tui -help

Flags:
  -config string
        Path to Antigravity hub JSON file (default auto-discovers in ~/.gemini/antigravity/daemon)
  -port int
        Override Antigravity HTTP port
  -token string
        Override Antigravity CSRF token
```

---

## ⌨️ Keybindings

| Keybinding | Action |
| :--- | :--- |
| `Tab` | Cycle focus between **Sidebar**, **Chat Input**, and **Chat History** |
| `↑` / `k` or `↓` / `j` | Select conversation in Sidebar / Scroll Chat Viewport |
| `Enter` | Send prompt to agent (when focused on input) |
| `Esc` | Stop running agent turn, or unfocus input |
| `r` | Refresh conversation trajectories list |
| `Ctrl+C` | Quit Antigravity TUI |

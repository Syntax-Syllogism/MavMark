# MK // Markdown Kanban

**MK** is a localhost Kanban board that uses your filesystem as its database. It's designed for developers who want the visualization of a Kanban board with the portability and version-control benefits of Markdown files.

## 🚀 Key Features

- **Single Binary**: No Node.js, no Docker, no external database. Just one file.
- **Filesystem Database**: Your tasks are stored as `.md` files in a `docs/` folder.
- **Real-time Sync**: Uses SSE (Server-Sent Events) to reflect file changes in the browser instantly (<200ms).
- **Edit & Preview**: Integrated Markdown editor and live preview within the web app.
- **Drag-and-Drop**: Move cards between columns to update their status automatically.
- **Tag Filtering**: Quickly focus on specific categories with a dynamic filter bar.
- **Git Ready**: Built-in detection for Git conflict markers.
- **Auto-Port**: Automatically finds an available port starting from `8080`.

## 🛠️ Setup & Installation

### 1. Download/Build

If you have Go installed, you can build the binary yourself. First, fetch the dependencies (`fsnotify` for filesystem watching and `yaml.v3` for parsing task frontmatter):

```bash
go mod tidy
```

Then build:

```bash
# macOS / Linux
go build -o mk main.go
```

```powershell
# Windows
go build -o mk.exe main.go
```

### 2. Install Globally (Optional)

Move the binary to your system path to use it in any project:

```bash
# macOS / Linux
sudo mv mk /usr/local/bin/mk
```

```powershell
# Windows — run PowerShell as Administrator
Move-Item mk.exe C:\Windows\System32\mk.exe
```

Alternatively on Windows, move it to any folder already in your `%PATH%`, or add a new folder via **System Properties → Environment Variables**.

### 3. Usage

Navigate to any project directory and run:

```bash
# macOS / Linux
mk
```

```powershell
# Windows — if installed globally
mk

# Windows — if running from the build directory
.\mk.exe
```

### Options

- **`-dir`**: Specify the directory containing your markdown tasks (default: `docs`).

  ```bash
  mk -dir my-tasks
  ```

- **`-port`**: Manually specify a port. If not provided, MK will try `8080` and then search for the next available port automatically.

  ```bash
  mk -port 9000
  ```

## 📂 Project Structure

- **`<tasks_dir>/*.md`**: Your task files. Each file should have a YAML frontmatter.
- **`.kanban/board.json`**: A generated index file used for fast UI rendering.

### Task Schema (Example)

Each `.md` file in the `docs/` folder should follow this format:

```markdown
---
status: TODO
epic: SETUP
tags: [backend, go]
---
# Task Title
Task description goes here...
```

## 🎨 Philosophy

1. **IDE First**: Your editor is the primary input; the web app is the primary visualization.
2. **Zero Dependency**: The tool should work on any machine without installing a runtime.
3. **Transparent State**: No hidden database. If you want to move a task, you can move the file or edit the text.

---
Built with Go and Vanilla JS.

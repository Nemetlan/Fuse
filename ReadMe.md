# Fuse ⚡

> **Fast, cross-platform desktop application for seamless file chunk reassembly.**

Fuse is a lightweight, high-performance desktop application built with Go and Fyne. It automatically detects and reconstructs split or chunked files back into their original single file with a clean, modern interface.

---

## ✨ Features

- **Drag & Drop UI**: Sleek, intuitive interface inspired by modern file-sharing utilities.
- **Smart Auto-Discovery**: Drop *any* single chunk, and Fuse automatically detects, sorts, and verifies all matching parts in the directory.
- **Safe Streaming Engine**: Memory-efficient buffered file streaming that handles multi-gigabyte files smoothly with real-time progress updates.
- **Atomic Merge & Verification**: Merges into a temporary file first and verifies total byte size before finalizing to eliminate file corruption.
- **Automatic Cleanup**: Optional automatic deletion of source chunk files upon successful merge.
- **Native Dark Mode**: Clean, minimal dark interface styled consistently across macOS, Windows, and Linux.

---

## 🛠️ Prerequisites

Before building or packaging Fuse, ensure your environment meets the following requirements:

1. **Go Environment**: [Go 1.20 or higher](https://go.dev/doc/install)
2. **Graphics Dependencies** (required by Cgo for Fyne rendering):
   - **macOS**: Xcode Command Line Tools (`xcode-select --install`)
   - **Linux**: `gcc`, `libgl1-mesa-dev`, `xorg-dev`
   - **Windows**: Working C compiler (e.g., [MinGW-w64](https://www.mingw-w64.org/))
3. **Fyne CLI**:
   ```bash
   go install fyne.io/fyne/v2/cmd/fyne@latest
   ```
4. **Environment PATH Setup**:
   Ensure Go's binary path is added to your shell configuration:
   ```bash
   echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.zshrc
   source ~/.zshrc
   ```

---

## 🚀 Building & Packaging

### 1. Local Run / Development Mode

Run the app directly in development mode without building a packaged binary:

```bash
go run .
```

---

### 2. Packaging for macOS (Darwin)

Build a native macOS `.app` bundle:

```bash
fyne package -os darwin -icon icon.png
```

*If `fyne` is not added to your global PATH:*
```bash
~/go/bin/fyne package -os darwin -icon icon.png
```

---

### 3. Packaging for Windows from macOS

Because Fyne uses OpenGL bindings (Cgo), cross-compilation requires proper C toolchains.

#### Method A: Using `fyne-cross` via Docker (Recommended)

1. Launch **Docker Desktop** on your Mac.
2. Install `fyne-cross`:
   ```bash
   go install github.com/fyne-io/fyne-cross@latest
   ```
3. Compile the Windows `.exe`:
   ```bash
   ~/go/bin/fyne-cross windows -icon icon.png
   ```
   *Output binary will be located in `fyne-cross/bin/windows/`.*

#### Method B: Direct MinGW Compilation (No Docker)

1. Install the MinGW toolchain:
   ```bash
   brew install mingw-w64
   ```
2. Build the standalone Windows binary:
   ```bash
   GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -ldflags "-H=windowsgui" -o Fuse.exe
   ```
   *(The `-ldflags "-H=windowsgui"` flag prevents a terminal console window from opening on Windows).*

---

### 4. Packaging for Linux

Build for Linux using `fyne-cross`:

```bash
~/go/bin/fyne-cross linux -icon icon.png
```

---

## 🔍 Troubleshooting

### `zsh: command not found: fyne`
Your Go binary folder is missing from your shell `PATH`. Either call the binary explicitly using `~/go/bin/fyne` or fix your permissions and configuration:
```bash
sudo chown $USER ~/.zshrc
echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.zshrc
source ~/.zshrc
```

### `[✗] engine binary not found in PATH`
`fyne-cross` cannot locate an active Docker container engine. Ensure **Docker Desktop** is open and running, then verify with `docker info`.

---

## 📂 Project Structure

```text
fuse/
├── main.go          # Application entry point & GUI definition
├── icon.png         # App icon resource
├── go.mod           # Go module declaration
├── go.sum           # Go module checksums
└── README.md        # Project documentation
```

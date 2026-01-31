# Allman Brace Style Formatting Tool (`go-formatter`)

A standalone Go-based CLI tool that automates formatting and linting for our specific stack. It wraps **ESLint** (for JS/TS) and **Prettier** + **Custom Logic** (for HTML Allman braces), all contained within a single executable.

## 📋 Prerequisites

Before running, ensure you have the following installed:

- **Go**: [Download & Install Go](https://go.dev/dl/).
- _Verify by running `go version` in your terminal._

- **Git**: [Download Git for Windows](https://git-scm.com/download/win).
- **Node.js**: (Optional but recommended) The tool will attempt to install its own internal Node dependencies if missing.

---

## ⚡ Installation (Windows)

The easiest way to install this tool is to build it directly into your user's Go binary folder. This folder is typically already in your system PATH, allowing you to run the command from anywhere.

1. **Clone the repository:**

```powershell
git clone https://github.com/caseycole589/go-formatter.git
cd go-formatter

```

2. **Build & Install:**
   Run this command in **PowerShell**. It will compile the executable and place it in your `go\bin` folder automatically.

```powershell
# Create the bin directory if it doesn't exist, then build
New-Item -ItemType Directory -Force -Path $env:USERPROFILE\go\bin
go build -o $env:USERPROFILE\go\bin\go-formatter.exe

```

3. **Verify Installation:**
   **Close your current terminal** and open a new one (this is required to refresh your Path). Then type:

```powershell
go-formatter

```

_If you see "Operating in: ..." or an error about paths, it is installed correctly!_

---

## 🚀 Usage

Navigate to any git repository on your machine and run the command.

### Run on current folder

```powershell
cd C:\Work\MyProject
go-formatter

```

### Run on a specific path

```powershell
go-formatter -path "C:\Users\You\Documents\ProjectX"

```

---

## 🛠️ What it Does

1. **Detects Changes**: It looks at your `git diff` to find changed files (relative to the parent branch).
2. **JS/TS Files**:

- Runs **ESLint** with our embedded config.
- Auto-fixes indentation, semi-colons, and spacing.

3. **HTML Files**:

- Runs **Prettier** (Tab width: 4).
- Runs a **Custom Formatter** to force Allman-style braces (braces on new lines) for directives like `@if`, `@switch`, etc.

---

## ⚙️ Development & Configuration

The configuration files (`.prettierrc`, `eslint.config.mjs`, `package.json`) are **embedded** inside the `.exe`. You do not need to copy them around.

If you need to update the rules:

1. Edit the files in the `configs/` folder of this repository.
2. Re-run the **Build & Install** command above to generate a new `.exe`.

### Folder Structure

```text
go-format/
├── main.go                # Main Go source code
├── go.mod                 # Go module definition
└── configs/               # Configs embedded into the binary
    ├── .prettierrc
    ├── eslint.config.mjs
    └── package.json

```

---

## ❓ Troubleshooting

**"The term 'go-formatter' is not recognized..."**

1. Ensure you restarted your terminal after installing.
2. Check if your Go bin folder is in your PATH. You can check if the file exists by running:

```powershell
Test-Path $env:USERPROFILE\go\bin\go-formatter.exe

```

- **True**: Your `go\bin` folder is missing from your Windows PATH environment variable.
- **False**: The build command failed. Check for errors.

**"ESLint/Prettier not found..."**
The tool attempts to install these automatically on the first run into a hidden folder: `~/.allman-formatter-tool`. If it gets stuck, you can manually delete that folder to force a fresh install:

```powershell
Remove-Item -Recurse -Force $env:USERPROFILE\.allman-formatter-tool

```

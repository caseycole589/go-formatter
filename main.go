package main

import (
    "embed"
    "flag"
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "runtime"
    "strings"
)

// --- EMBEDDED CONFIGURATION ---

// This directive bundles the files inside the 'configs' folder into the binary
//go:embed configs/*
var configFiles embed.FS

var repoPath string
var toolHome string 

func main() {
    var inputPath string
    flag.StringVar(&inputPath, "path", ".", "Path to the git repository")
    flag.Parse()

    //  Setup Repo Path
    absPath, err := filepath.Abs(inputPath)
    if err != nil {
        log.Fatalf("Error resolving path: %v", err)
    }
    repoPath = absPath
    if _, err := os.Stat(repoPath); os.IsNotExist(err) {
        log.Fatalf("Directory does not exist: %s", repoPath)
    }

    fmt.Printf("Operating in: %s\n", repoPath)

    // Setup the Linter Environment
    setupToolEnvironment()

    // Git Logic
    currentBranch := getCommandOutput("git", "branch", "--show-current")
    if currentBranch == "" {
        log.Fatalf("Could not detect current branch.")
    }

    parentBranch := findForkPoint(currentBranch)
    if !isValidRef(parentBranch) {
        fmt.Printf("Parent '%s' not found. Falling back to 'main'.\n", parentBranch)
        parentBranch = "main"
    }

    fmt.Printf("Calculating changes: %s...%s\n", parentBranch, currentBranch)

    cmd := exec.Command("git", "diff", "--name-only", fmt.Sprintf("%s...HEAD", parentBranch))
    cmd.Dir = repoPath
    output, err := cmd.CombinedOutput()
    if err != nil {
        log.Fatalf("Error running git diff: %v", err)
    }

    // 4. Run the processors
    processChanges(string(output))
}

// --- TOOL ENVIRONMENT SETUP ---

func setupToolEnvironment() {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        log.Fatalf("Could not find user home directory: %v", err)
    }

    toolHome = filepath.Join(homeDir, ".insipp-linter-tool")
    if err := os.MkdirAll(toolHome, 0755); err != nil {
        log.Fatalf("Failed to create tool directory: %v", err)
    }

    // Helper to extract embedded files to the user's disk
    extractFile := func(embedPath, destName string) {
        content, err := configFiles.ReadFile(embedPath)
        if err != nil {
            log.Fatalf("Failed to read embedded config %s: %v", embedPath, err)
        }
        destPath := filepath.Join(toolHome, destName)
        if err := os.WriteFile(destPath, content, 0644); err != nil {
            log.Fatalf("Failed to write config %s: %v", destName, err)
        }
    }

    // Always overwrite configs to keep them up to date with the binary
    extractFile("configs/eslint.config.mjs", "eslint.config.mjs")
    extractFile("configs/.prettierrc", ".prettierrc")

    // Check if we need to install/update dependencies
    pkgDest := filepath.Join(toolHome, "package.json")
    prettierBin := filepath.Join(toolHome, "node_modules", ".bin", "prettier")
    if runtime.GOOS == "windows" {
        prettierBin += ".cmd"
    }

    _, pkgErr := os.Stat(pkgDest)
    _, binErr := os.Stat(prettierBin)

    needsInstall := os.IsNotExist(pkgErr) || os.IsNotExist(binErr)

    if needsInstall {
        fmt.Println("Updating linter environment (installing Prettier/ESLint)...")

        // Write package.json only when installing to trigger updates if needed
        extractFile("configs/package.json", "package.json")

        npmCmd := "npm"
        if runtime.GOOS == "windows" {
            npmCmd = "npm.cmd"
        }

        cmd := exec.Command(npmCmd, "install")
        cmd.Dir = toolHome
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr

        if err := cmd.Run(); err != nil {
            log.Fatalf("Failed to install linter dependencies: %v", err)
        }
        fmt.Println("Tool environment ready.")
    }
}

// --- FILE PROCESSING ---

func processChanges(rawOutput string) {
    lines := strings.Split(strings.TrimSpace(rawOutput), "\n")

    var eslintFiles []string
    var htmlFiles []string

    for _, f := range lines {
        f = strings.TrimSpace(f)
        if f == "" {
            continue
        }
        fullPath := filepath.Join(repoPath, f)

        if _, err := os.Stat(fullPath); os.IsNotExist(err) {
            continue
        }

        ext := strings.ToLower(filepath.Ext(f))

        switch ext {
        case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
            eslintFiles = append(eslintFiles, fullPath)
        case ".html":
            htmlFiles = append(htmlFiles, fullPath)
        }
    }

    if len(eslintFiles) > 0 {
        runEslint(eslintFiles)
    } else {
        fmt.Println("No JS/TS files to lint.")
    }

    if len(htmlFiles) > 0 {
        runHtmlProcessing(htmlFiles)
    } else {
        fmt.Println("No HTML files to process.")
    }
}

func runEslint(files []string) {
    fmt.Printf("Running ESLint --fix on %d file(s)...\n", len(files))

    eslintBin := filepath.Join(toolHome, "node_modules", ".bin", "eslint")
    if runtime.GOOS == "windows" {
        eslintBin += ".cmd"
    }

    configPath := filepath.Join(toolHome, "eslint.config.mjs")
    args := []string{"--config", configPath, "--fix"}
    args = append(args, files...)

    cmd := exec.Command(eslintBin, args...)
    cmd.Dir = repoPath
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        fmt.Println("\nESLint finished with issues (or fixed code).")
    } else {
        fmt.Println("\nESLint finished successfully.")
    }
}

func runHtmlProcessing(files []string) {
    fmt.Printf("Processing %d HTML file(s) (Prettier + Allman Braces)...\n", len(files))

    // 1. Run Prettier First
    prettierBin := filepath.Join(toolHome, "node_modules", ".bin", "prettier")
    if runtime.GOOS == "windows" {
        prettierBin += ".cmd"
    }

    // Use the config file extracted to the tool home
    configPath := filepath.Join(toolHome, ".prettierrc")
    
    args := []string{"--write", "--config", configPath}
    args = append(args, files...)

    cmd := exec.Command(prettierBin, args...)
    cmd.Dir = repoPath
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        fmt.Printf("Prettier encountered a warning/error (continuing to custom formatting): %v\n", err)
    }

    // Custom Regex for Allman Braces on @directives
    regexStr := `(?m)^(\s*)(@(?:if|else|elseif|switch|for|foreach|while)\b(?:[^{]*))\s*\{`
    re := regexp.MustCompile(regexStr)

    for _, file := range files {
        content, err := os.ReadFile(file)
        if err != nil {
            fmt.Printf("Error reading %s: %v\n", file, err)
            continue
        }

        contentStr := string(content)
        newContent := re.ReplaceAllString(contentStr, "$1$2\n$1{")

        if newContent != contentStr {
            if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
                fmt.Printf("Error writing %s: %v\n", file, err)
            }
        }
    }
    fmt.Println("HTML processing finished.")
}

// --- UTILITIES ---

func findForkPoint(currentBranch string) string {
    reflogOut := getCommandOutput("git", "reflog", "--date=iso")
    lines := strings.Split(reflogOut, "\n")
    for _, line := range lines {
        if strings.Contains(line, "moving from ") && strings.Contains(line, fmt.Sprintf(" to %s", currentBranch)) {
            parts := strings.Split(line, "moving from ")
            if len(parts) > 1 {
                remainder := parts[1]
                toParts := strings.Split(remainder, " to ")
                candidate := strings.TrimSpace(toParts[0])
                if isSameBranch(candidate, currentBranch) {
                    continue
                }
                return candidate
            }
        }
    }
    candidates := []string{"main", "master", "develop", "origin/main", "origin/master"}
    for _, c := range candidates {
        if isValidRef(c) {
            if isSameBranch(c, currentBranch) {
                continue
            }
            return c
        }
    }
    return "main"
}

func isSameBranch(candidate, current string) bool {
    if candidate == current || candidate == "origin/"+current {
        return true
    }
    if strings.HasSuffix(candidate, "/"+current) {
        return true
    }
    return false
}

func isValidRef(ref string) bool {
    cmd := exec.Command("git", "rev-parse", "--verify", ref)
    cmd.Dir = repoPath
    return cmd.Run() == nil
}

func getCommandOutput(name string, args ...string) string {
    cmd := exec.Command(name, args...)
    cmd.Dir = repoPath
    out, err := cmd.CombinedOutput()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}
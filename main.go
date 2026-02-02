package main

import (
    "embed"
    "flag"
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
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

    fmt.Printf("2. Operating in : %s\n", repoPath)

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

    // Process each file with custom formatting
    for _, file := range files {
        content, err := os.ReadFile(file)
        if err != nil {
            fmt.Printf("Error reading %s: %v\n", file, err)
            continue
        }

        contentStr := string(content)
        newContent := formatAngularTemplate(contentStr)

        if newContent != contentStr {
            if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
                fmt.Printf("Error writing %s: %v\n", file, err)
            }
        }
    }
    fmt.Println("HTML processing finished.")
}

// Replace your existing formatAngularTemplate function with this implementation.
// This properly handles:
// - Nested parentheses like adminTypes()
// - @else and @else if patterns
// - Multiple closing braces on one line (} } or } } })
// - Preserves {{ }} interpolation
// - Preserves HTML comments

const indentUnit = "    " // 4 spaces - adjust if you use tabs or different spacing



func formatAngularTemplate(content string) string {
    lines := strings.Split(content, "\n")
    var result []string

    depth := 0
    inComment := false

    for _, originalLine := range lines {
        trimmed := strings.TrimSpace(originalLine)
        originalIndent := extractIndent(originalLine)

        if trimmed == "" {
            result = append(result, "")
            continue
        }

        // Track multi-line HTML comments - preserve exactly
        if strings.Contains(trimmed, "<!--") && !strings.Contains(trimmed, "-->") {
            inComment = true
            result = append(result, originalLine)
            continue
        }
        if inComment {
            result = append(result, originalLine)
            if strings.Contains(trimmed, "-->") {
                inComment = false
            }
            continue
        }

        // Check if this line needs expansion
        needsExpand := (strings.Contains(trimmed, "@") && isControlFlowLine(trimmed)) ||
            strings.Contains(trimmed, "} }")

        if !needsExpand {
            // Check for standalone }
            if trimmed == "}" {
                depth--
                if depth < 0 {
                    depth = 0
                }
                extraIndent := strings.Repeat(indentUnit, depth)
                result = append(result, extraIndent+originalIndent+trimmed)
                continue
            }

            // Regular line - add depth-based indent
            extraIndent := strings.Repeat(indentUnit, depth)
            result = append(result, extraIndent+originalIndent+trimmed)
            continue
        }

        // Expand this line
        expanded := expandLineWithIndent(trimmed, originalIndent, depth)

        for _, expLine := range expanded.lines {
            result = append(result, expLine)
        }

        depth = expanded.finalDepth
    }

    return strings.Join(result, "\n")
}

type expandResult struct {
    lines      []string
    finalDepth int
}

func isControlFlowLine(trimmed string) bool {
    if (strings.Contains(trimmed, "@for") || strings.Contains(trimmed, "@if") ||
        strings.Contains(trimmed, "@else") || strings.Contains(trimmed, "@switch")) &&
        strings.Contains(trimmed, "{") {
        return true
    }
    if strings.Contains(trimmed, "} @") {
        return true
    }
    return false
}

func expandLineWithIndent(trimmed, originalIndent string, startDepth int) expandResult {
    var result []string
    var currentLine strings.Builder

    depth := startDepth
    localDepth := 0

    i := 0
    for i < len(trimmed) {
        ch := trimmed[i]

        // Handle {{ interpolation
        if ch == '{' && i+1 < len(trimmed) && trimmed[i+1] == '{' {
            currentLine.WriteString("{{")
            i += 2
            for i < len(trimmed) {
                if trimmed[i] == '}' && i+1 < len(trimmed) && trimmed[i+1] == '}' {
                    currentLine.WriteString("}}")
                    i += 2
                    break
                }
                currentLine.WriteByte(trimmed[i])
                i++
            }
            continue
        }

        // Handle @directive
        if ch == '@' && isControlFlowDirective(trimmed[i:]) {
            flushWithDepth(&result, &currentLine, originalIndent, depth+localDepth)
            directive, newPos := extractDirective(trimmed, i)
            result = append(result, depthIndent(originalIndent, depth+localDepth)+directive)
            i = newPos
            for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
                i++
            }
            if i < len(trimmed) && trimmed[i] == '{' {
                result = append(result, depthIndent(originalIndent, depth+localDepth)+"{")
                localDepth++
                i++
                for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
                    i++
                }
            }
            continue
        }

        // Handle }
        if ch == '}' {
            flushWithDepth(&result, &currentLine, originalIndent, depth+localDepth)
            localDepth--
            if depth+localDepth < 0 {
                localDepth = -depth
            }
            result = append(result, depthIndent(originalIndent, depth+localDepth)+"}")
            i++
            for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
                i++
            }
            continue
        }

        // Handle standalone {
        if ch == '{' {
            flushWithDepth(&result, &currentLine, originalIndent, depth+localDepth)
            result = append(result, depthIndent(originalIndent, depth+localDepth)+"{")
            localDepth++
            i++
            for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
                i++
            }
            continue
        }

        currentLine.WriteByte(ch)
        i++
    }

    flushWithDepth(&result, &currentLine, originalIndent, depth+localDepth)

    if len(result) == 0 {
        result = []string{depthIndent(originalIndent, depth) + trimmed}
    }

    return expandResult{
        lines:      result,
        finalDepth: depth + localDepth,
    }
}

func depthIndent(originalIndent string, depth int) string {
    if depth < 0 {
        depth = 0
    }
    return strings.Repeat(indentUnit, depth) + originalIndent
}

func flushWithDepth(result *[]string, currentLine *strings.Builder, originalIndent string, depth int) {
    content := strings.TrimSpace(currentLine.String())
    if content != "" {
        *result = append(*result, depthIndent(originalIndent, depth)+content)
    }
    currentLine.Reset()
}

func isControlFlowDirective(s string) bool {
    directives := []string{"@if", "@else if", "@else", "@switch", "@case", "@default", "@for", "@empty"}
    for _, d := range directives {
        if strings.HasPrefix(s, d) {
            if len(s) == len(d) {
                return true
            }
            next := s[len(d)]
            if next == ' ' || next == '(' || next == '{' || next == '\n' || next == '\t' {
                return true
            }
        }
    }
    return false
}

func extractDirective(line string, start int) (string, int) {
    i := start
    parenDepth := 0
    inParens := false

    for i < len(line) {
        ch := line[i]
        if ch == '(' {
            parenDepth++
            inParens = true
        } else if ch == ')' {
            parenDepth--
            if parenDepth == 0 && inParens {
                return line[start : i+1], i + 1
            }
        } else if ch == '{' && parenDepth == 0 {
            return strings.TrimSpace(line[start:i]), i
        }
        i++
    }
    return strings.TrimSpace(line[start:]), len(line)
}

func extractIndent(line string) string {
    for i, ch := range line {
        if ch != ' ' && ch != '\t' {
            return line[:i]
        }
    }
    return ""
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
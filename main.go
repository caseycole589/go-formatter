package main

import (
    "flag"
    "fmt"
    "log"
    "os/exec"
    "strings"
)

var repoPath string

func main() {
    // Parse Flags
    // Default value is "." (current directory)
    flag.StringVar(&repoPath, "path", ".", "Path to the git repository")
    flag.Parse()

    fmt.Printf("Operating in: %s\n", repoPath)

    // Identify where we are right now
    currentBranch := getCommandOutput("git", "branch", "--show-current")
    if currentBranch == "" {
        log.Fatalf("Could not detect current branch at %s. Are you in a git repo?", repoPath)
    }

    // Find the "fork point"
    parentBranch := findForkPoint(currentBranch)

    // Verify the parent exists before running diff
    if !isValidRef(parentBranch) {
        fmt.Printf("Parent '%s' not found. Falling back to 'main'.\n", parentBranch)
        parentBranch = "main"
    }

    fmt.Printf("Calculating changes: %s...%s\n", parentBranch, currentBranch)

    // We use the triple-dot (...) to find the common ancestor
    cmd := exec.Command("git", "diff", "--name-only", fmt.Sprintf("%s...HEAD", parentBranch))
    cmd.Dir = repoPath

    output, err := cmd.CombinedOutput()
    if err != nil {
        log.Fatalf("Error running git diff: %v\nOutput: %s", err, string(output))
    }

    printResults(string(output))
}

// --- HELPER FUNCTIONS ---

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

                fmt.Printf("Auto-detected parent (from checkout history): '%s'\n", candidate)
                return candidate
            }
        }
    }

    fmt.Println("No checkout history found. Checking standard base branches...")
    candidates := []string{"main", "master", "develop", "origin/main", "origin/master", "origin/develop"}
    
    for _, c := range candidates {
        if isValidRef(c) {
            if isSameBranch(c, currentBranch) {
                continue
            }
            fmt.Printf("Defaulting to base branch: '%s'\n", c)
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

func printResults(rawOutput string) {
    files := strings.TrimSpace(rawOutput)
    if files == "" {
        fmt.Println("No files changed.")
        return
    }
    fileList := strings.Split(files, "\n")
    fmt.Printf("Found %d changed file(s):\n", len(fileList))
    for _, file := range fileList {
        fmt.Printf("- %s\n", file)
    }
}
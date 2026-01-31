package main

import (
    "fmt"
    "log"
    "os/exec"
    "strings"
)

// Path to the repository (one level up)
const repoPath = "../NAVY-EBS-INSIPP-Digitalization"

func main() {
    // Identify where we are right now
    currentBranch := getCommandOutput("git", "branch", "--show-current")
    if currentBranch == "" {
        log.Fatal("Could not detect current branch. Are you in a git repo?")
    }

    // Find the "fork point" (the branch we came from)
    parentBranch := findForkPoint(currentBranch)

    // Verify the parent exists before running diff
    if !isValidRef(parentBranch) {
        // If the specific parent is gone/invalid, fall back to a safe default
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

func findForkPoint(currentBranch string) string {
    // Strategy A: Check the Reflog for the "checkout" event
    // We look for: "checkout: moving from <PARENT> to <CURRENT>"
    // We scan the HEAD reflog because it records the actual switching of branches.
    reflogOut := getCommandOutput("git", "reflog", "--date=iso")
    lines := strings.Split(reflogOut, "\n")

    for _, line := range lines {
        // Look for the pattern "moving from ... to currentBranch"
        if strings.Contains(line, fmt.Sprintf("moving from ")) && strings.Contains(line, fmt.Sprintf(" to %s", currentBranch)) {
            
            // Parse out the "from" branch
            parts := strings.Split(line, "moving from ")
            if len(parts) > 1 {
                remainder := parts[1]
                toParts := strings.Split(remainder, " to ")
                candidate := strings.TrimSpace(toParts[0])

                // FILTER: Ignore if the parent is effectively the same as the current branch.
                // This handles "moving from origin/feature to feature" or "feature to feature"
                if isSameBranch(candidate, currentBranch) {
                    continue
                }

                fmt.Printf("Auto-detected parent (from checkout history): '%s'\n", candidate)
                return candidate
            }
        }
    }

    // Strategy B: If Reflog fails (e.g. fresh clone), pick the best "Trunk" branch.
    // We check which standard branch exists and is "closest".
    fmt.Println("No checkout history found. Checking standard base branches...")
    
    // List of likely parents to check in order of preference
    candidates := []string{"main", "master", "develop", "origin/main", "origin/master", "origin/develop"}
    
    for _, c := range candidates {
        if isValidRef(c) {
            // Quick check: ensure we aren't comparing main...main
            if isSameBranch(c, currentBranch) {
                continue
            }
            fmt.Printf("Defaulting to base branch: '%s'\n", c)
            return c
        }
    }

    return "main" // Absolute fallback
}

// Helper to decide if two branch names refer to the same logical stream
// e.g. "feature" is the same as "origin/feature"
func isSameBranch(candidate, current string) bool {
    if candidate == current {
        return true
    }
    if candidate == "origin/"+current {
        return true
    }
    // Handle full refs if necessary (refs/heads/...)
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
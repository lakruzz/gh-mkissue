package mkissue

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type IssueMetadata struct {
	Title     string   `yaml:"title"`
	Assignees []string `yaml:"assign,omitempty"`
	Labels    []Label  `yaml:"labels,omitempty"`
	Milestone string   `yaml:"milestone,omitempty"`
	Projects  []string `yaml:"projects,omitempty"`
}

type Label struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color,omitempty"`
	Desc  string `yaml:"desc,omitempty"`
}

var repoNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
var frontmatterDelimiter = regexp.MustCompile(`(?m)^---[ \t]*\r?$`)

func Run(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: utils mkissue create --file <file.issue.md>")
		os.Exit(1)
	}

	issueFile := args[0]
	if err := RunWithFile(issueFile, "", "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// RunWithFile processes a single issue file and returns an error instead of exiting.
// This function is compatible with Cobra command error handling.
// If branch is provided, the file will be read from that git branch.
// If gist is provided, the file will be read from that gist.
// If repo is provided (owner/repo format), the file will be read from that GitHub repository.
func RunWithFile(issueFile, branch, gist, repo string) error {
	return Create(CreateOptions{File: issueFile, Branch: branch, Gist: gist, Repo: repo})
}

// CreateOptions selects the issue source and post-creation behavior.
type CreateOptions struct {
	File         string
	Branch       string
	Gist         string
	Repo         string
	Clean        bool
	CreateLabels bool
}

// Create creates an issue from the selected source.
func Create(options CreateOptions) error {
	issueFile, branch, gist, repo := options.File, options.Branch, options.Gist, options.Repo
	if options.Clean && (branch != "" || gist != "" || repo != "") {
		return fmt.Errorf("--clean requires a local file source")
	}
	var content []byte
	var err error
	var snapshot sourceSnapshot

	// Read the file from the specified source
	if gist != "" {
		content, err = readFileFromGist(issueFile, gist)
		if err != nil {
			return fmt.Errorf("failed to read file from gist '%s': %w", gist, err)
		}
	} else if repo != "" {
		content, err = readFileFromRepo(issueFile, repo, branch)
		if err != nil {
			return fmt.Errorf("failed to read file from repo '%s': %w", repo, err)
		}
	} else if branch != "" {
		content, err = readFileFromBranch(issueFile, branch)
		if err != nil {
			return fmt.Errorf("failed to read file from branch '%s': %w", branch, err)
		}
	} else if options.Clean {
		content, snapshot, err = readLocalSource(issueFile)
		if err != nil {
			return fmt.Errorf("read local source %q: %w", issueFile, err)
		}
	} else {
		content, err = os.ReadFile(issueFile)
		if err != nil {
			return fmt.Errorf("file '%s' not found: %w", issueFile, err)
		}
	}

	metadata, body, err := parseDraftFile(string(content))
	if err != nil {
		return err
	}

	run := newGitHubRunner("")
	if err := validateMilestone(metadata.Milestone, "", run); err != nil {
		return err
	}
	if err := prepareIssueLabels(metadata.Labels, nil, options.CreateLabels, run); err != nil {
		return err
	}

	// Create the issue
	if err := createIssue(metadata, body); err != nil {
		return fmt.Errorf("error creating issue: %w", err)
	}

	fmt.Println("Issue created successfully!")
	if options.Clean {
		if err := cleanSource(snapshot); err != nil {
			return fmt.Errorf("issue created, but source cleanup failed: %w", err)
		}
	}
	return nil
}

// readFileFromBranch reads a file from a specific git branch without checking it out.
// It uses `git show <branch>:<file>` to retrieve the file content.
func readFileFromBranch(filePath, branch string) ([]byte, error) {
	// Basic validation: ensure branch name doesn't contain null bytes or newlines
	// which could cause issues with git commands
	if strings.ContainsAny(branch, "\x00\n\r") {
		return nil, fmt.Errorf("invalid branch name: contains prohibited characters")
	}
	if strings.ContainsAny(filePath, "\x00\n\r") {
		return nil, fmt.Errorf("invalid file path: contains prohibited characters")
	}

	// Use git show to read the file from the specified branch
	// Note: exec.Command passes arguments separately, not through shell, preventing injection
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", branch, filePath))
	output, err := cmd.Output()
	if err != nil {
		// Check if it's an exit error and provide more context
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to read file from branch: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to read file from branch: %w", err)
	}
	return output, nil
}

// readFileFromGist reads a file from a GitHub gist using the gh CLI.
// It uses `gh gist view <gist-id> -f <filename> -r` to retrieve the file content.
func readFileFromGist(fileName, gistID string) ([]byte, error) {
	// Validate gist ID - GitHub gist IDs are hexadecimal strings (32 characters)
	// Using a positive pattern for security and maintainability
	gistIDPattern := regexp.MustCompile(`^[a-f0-9]{32}$`)
	if !gistIDPattern.MatchString(gistID) {
		return nil, fmt.Errorf("invalid gist ID: must be a 32-character hexadecimal string")
	}

	// Validate file name - allow alphanumeric, dots, hyphens, underscores
	// GitHub gists use flat file structure (no subdirectories), so reject path separators
	fileNamePattern := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !fileNamePattern.MatchString(fileName) {
		return nil, fmt.Errorf("invalid file name: only alphanumeric characters, dots, hyphens, and underscores are allowed")
	}

	// Use gh gist view to read the file from the specified gist
	// Note: exec.Command passes arguments separately, not through shell, preventing injection
	cmd := exec.Command("gh", "gist", "view", gistID, "-f", fileName, "-r")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's an exit error and provide more context
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to read file from gist: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to read file from gist: %w", err)
	}
	return output, nil
}

// readFileFromRepo reads a file from a GitHub repository using the gh CLI.
// It uses `gh api repos/{owner}/{repo}/contents/{path}` with optional ref to retrieve the file content.
// If branch is empty, the repository's default branch is used.
func readFileFromRepo(filePath, repo, branch string) ([]byte, error) {
	// Validate repo format: must be "owner/repo"
	if !repoNamePattern.MatchString(repo) {
		return nil, fmt.Errorf("invalid repository format: must be 'owner/repo'")
	}

	// Validate file path
	if strings.ContainsAny(filePath, "\x00\n\r") {
		return nil, fmt.Errorf("invalid file path: contains prohibited characters")
	}

	// Validate branch name if provided
	if branch != "" && strings.ContainsAny(branch, "\x00\n\r") {
		return nil, fmt.Errorf("invalid branch name: contains prohibited characters")
	}

	// Normalize file path to remove leading ./ and other path inconsistencies
	// This ensures the GitHub API receives a proper path
	cleanPath := filepath.Clean(filePath)
	if strings.HasPrefix(cleanPath, "."+string(filepath.Separator)) {
		cleanPath = cleanPath[2:]
	}

	// Build the gh api command to fetch raw file content
	endpoint := fmt.Sprintf("repos/%s/contents/%s", repo, cleanPath)
	args := []string{"api", "-X", "GET", "-H", "Accept: application/vnd.github.raw", endpoint}

	if branch != "" {
		args = append(args, "-f", fmt.Sprintf("ref=%s", branch))
	}

	// Note: exec.Command passes arguments separately, not through shell, preventing injection
	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to read file from repo: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to read file from repo: %w", err)
	}
	return output, nil
}

func parseIssueFile(content string) (*IssueMetadata, string, error) {
	delimiters := frontmatterDelimiter.FindAllStringIndex(content, 2)
	if len(delimiters) < 2 || delimiters[0][0] != 0 {
		return nil, "", fmt.Errorf("invalid format: frontmatter not found")
	}

	frontmatter := strings.TrimSpace(content[delimiters[0][1]:delimiters[1][0]])
	body := strings.TrimSpace(content[delimiters[1][1]:])

	metadata := &IssueMetadata{}
	if err := yaml.Unmarshal([]byte(frontmatter), metadata); err == nil {
		for index, assignee := range metadata.Assignees {
			metadata.Assignees[index] = strings.Trim(assignee, "@")
		}
		return metadata, body, nil
	}
	metadata = &IssueMetadata{}

	// Parse frontmatter
	lines := strings.Split(frontmatter, "\n")
	var i int
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "title:") {
			metadata.Title = extractValue(trimmed, "title:")
		} else if strings.HasPrefix(trimmed, "assign:") {
			metadata.Assignees, i = parseListField(lines, i, "assign:")
		} else if strings.HasPrefix(trimmed, "labels:") {
			metadata.Labels, i = parseLabels(lines, i)
		} else if strings.HasPrefix(trimmed, "milestone:") {
			metadata.Milestone = extractValue(trimmed, "milestone:")
		} else if strings.HasPrefix(trimmed, "projects:") {
			metadata.Projects, i = parseListField(lines, i, "projects:")
		}

		i++
	}

	return metadata, body, nil
}

func extractValue(line, prefix string) string {
	value := strings.TrimPrefix(line, prefix)
	value = strings.TrimSpace(value)

	// Strip inline comments (but preserve # inside quotes)
	value = stripYAMLComment(value)

	value = strings.Trim(value, `"'`)
	return value
}

// stripYAMLComment removes YAML comments from a value, preserving # inside quotes
func stripYAMLComment(value string) string {
	inSingleQuote := false
	inDoubleQuote := false
	var escaped bool

	for i, ch := range value {
		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		// If we find # outside of quotes, strip from here onwards
		if ch == '#' && !inSingleQuote && !inDoubleQuote {
			return strings.TrimSpace(value[:i])
		}
	}

	return strings.TrimSpace(value)
}

func parseListField(lines []string, startIdx int, prefix string) ([]string, int) {
	line := lines[startIdx]
	trimmed := strings.TrimSpace(line)

	// Check for inline array format
	if strings.Contains(trimmed, "[") {
		content := strings.TrimPrefix(trimmed, prefix)
		content = strings.TrimSpace(content)

		// Strip comments before processing
		content = stripYAMLComment(content)
		content = strings.Trim(content, "[]")

		var items []string
		for _, item := range strings.Split(content, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"'@`)
			if item != "" {
				items = append(items, item)
			}
		}
		return items, startIdx
	}

	// Multi-line format
	var items []string
	i := startIdx + 1
	for i < len(lines) {
		line := lines[i]
		if !strings.HasPrefix(strings.TrimSpace(line), "-") {
			// Check if it's a new field
			if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				break
			}
			i++
			continue
		}

		item := strings.TrimSpace(line)
		item = strings.TrimPrefix(item, "-")
		item = strings.TrimSpace(item)

		// Strip comments from list items
		item = stripYAMLComment(item)

		item = strings.Trim(item, `"'@`)
		if item != "" {
			items = append(items, item)
		}
		i++
	}
	return items, i - 1
}

func parseLabels(lines []string, startIdx int) ([]Label, int) {
	var labels []Label
	i := startIdx + 1

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if we've reached a new top-level field
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(line, ":") {
			break
		}

		if strings.HasPrefix(trimmed, "- name:") {
			label := Label{
				Name: extractValue(trimmed, "- name:"),
			}

			// Look ahead for color and desc
			i++
			for i < len(lines) {
				nextLine := lines[i]
				nextTrimmed := strings.TrimSpace(nextLine)

				if strings.HasPrefix(nextTrimmed, "- name:") {
					i--
					break
				}

				if !strings.HasPrefix(nextLine, " ") && !strings.HasPrefix(nextLine, "\t") {
					i--
					break
				}

				if strings.HasPrefix(nextTrimmed, "color:") {
					label.Color = extractValue(nextTrimmed, "color:")
				} else if strings.HasPrefix(nextTrimmed, "desc:") {
					label.Desc = extractValue(nextTrimmed, "desc:")
				}

				i++
			}

			labels = append(labels, label)
		}

		i++
	}

	return labels, i - 1
}

func createIssue(metadata *IssueMetadata, body string) error {
	args := []string{"issue", "create", "--title", metadata.Title}

	// Add body
	if body != "" {
		// Write body to temp file
		tmpFile, err := os.CreateTemp("", "issue-body-*.md")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(body); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("failed to write body: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("failed to close body file: %w", err)
		}

		args = append(args, "--body-file", tmpFile.Name())
	} else {
		args = append(args, "--body", "")
	}

	// Add assignees
	for _, assignee := range metadata.Assignees {
		if assignee == "me" {
			assignee = "@me"
		}
		encoded, err := csvArgument(assignee)
		if err != nil {
			return err
		}
		args = append(args, "--assignee", encoded)
	}

	// Add labels
	for _, label := range metadata.Labels {
		encoded, err := csvArgument(label.Name)
		if err != nil {
			return err
		}
		args = append(args, "--label", encoded)
	}

	// Add milestone
	if metadata.Milestone != "" {
		args = append(args, "--milestone", metadata.Milestone)
	}

	// Add projects
	for _, project := range metadata.Projects {
		encoded, err := csvArgument(project)
		if err != nil {
			return err
		}
		args = append(args, "--project", encoded)
	}

	fmt.Println("Creating issue...")
	return runGhCommand(args)
}

func runGhCommand(args []string) error {
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh command failed: %w\nStderr: %s", err, stderr.String())
	}

	return nil
}

package mkissue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type pulledIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	ProjectItems []struct {
		Title string `json:"title"`
	} `json:"projectItems"`
}

// ListOpenIssueNumbers returns all open issue numbers in the current repository.
func ListOpenIssueNumbers() ([]int, error) {
	rootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("find repository root: %w", err)
	}
	command := exec.Command("gh", "api", "--paginate", "--slurp", "repos/{owner}/{repo}/issues?state=open&per_page=100")
	command.Dir = strings.TrimSpace(string(rootOutput))
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && len(exitError.Stderr) > 0 {
			return nil, fmt.Errorf("list open issues: %s: %w", strings.TrimSpace(string(exitError.Stderr)), err)
		}
		return nil, fmt.Errorf("list open issues: %w", err)
	}
	return parseOpenIssuePages(output)
}

func parseOpenIssuePages(data []byte) ([]int, error) {
	var pages [][]struct {
		Number      int             `json:"number"`
		PullRequest json.RawMessage `json:"pull_request"`
	}
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("decode open issues: %w", err)
	}
	if pages == nil {
		return nil, fmt.Errorf("decode open issues: expected an array of pages")
	}
	var numbers []int
	seen := make(map[int]bool)
	for _, page := range pages {
		for _, issue := range page {
			if len(issue.PullRequest) > 0 && string(issue.PullRequest) != "null" {
				continue
			}
			if issue.Number <= 0 {
				return nil, fmt.Errorf("open issue response contains an invalid issue number")
			}
			if !seen[issue.Number] {
				seen[issue.Number] = true
				numbers = append(numbers, issue.Number)
			}
		}
	}
	return numbers, nil
}

// Pull saves an existing issue from the current repository as a local issue file.
func Pull(number int) (string, error) {
	return PullTo(number, "", false)
}

// PullTo saves an issue in the specified directory, optionally replacing its draft.
func PullTo(number int, directory string, force bool) (string, error) {
	if number <= 0 {
		return "", fmt.Errorf("issue number must be a positive integer")
	}
	defaultDirectory := directory == ""
	if directory != "" {
		var err error
		directory, err = filepath.Abs(directory)
		if err != nil {
			return "", fmt.Errorf("resolve output directory: %w", err)
		}
	}
	rootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find repository root: %w", err)
	}
	root := strings.TrimSpace(string(rootOutput))
	// #nosec G204 -- the only variable argument is a validated positive integer
	command := exec.Command("gh", "issue", "view", strconv.Itoa(number), "--json", "number,title,body,assignees,labels,milestone,projectItems")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && len(exitError.Stderr) > 0 {
			return "", fmt.Errorf("fetch issue %d: %s: %w", number, strings.TrimSpace(string(exitError.Stderr)), err)
		}
		return "", fmt.Errorf("fetch issue %d: %w", number, err)
	}
	var issue pulledIssue
	if err := json.Unmarshal(output, &issue); err != nil {
		return "", fmt.Errorf("decode issue %d: %w", number, err)
	}
	if issue.Number != number || issue.Title == "" {
		return "", fmt.Errorf("issue %d response is missing its number or title", number)
	}
	if directory == "" {
		directory = filepath.Join(root, "issues")
	}
	return savePulledIssueTo(directory, issue, force, defaultDirectory)
}

func savePulledIssue(root string, issue pulledIssue) (string, error) {
	return savePulledIssueTo(filepath.Join(root, "issues"), issue, false, true)
}

func savePulledIssueTo(directory string, issue pulledIssue, force, defaultDirectory bool) (string, error) {
	metadata := IssueMetadata{Title: issue.Title}
	for _, assignee := range issue.Assignees {
		metadata.Assignees = append(metadata.Assignees, assignee.Login)
	}
	for _, label := range issue.Labels {
		metadata.Labels = append(metadata.Labels, Label{Name: label.Name})
	}
	if issue.Milestone != nil {
		metadata.Milestone = issue.Milestone.Title
	}
	for _, project := range issue.ProjectItems {
		metadata.Projects = append(metadata.Projects, project.Title)
	}
	frontmatter, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("encode issue frontmatter: %w", err)
	}
	if err := ensurePullDirectory(directory, defaultDirectory); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read issues directory: %w", err)
	}
	prefix := strconv.Itoa(issue.Number) + "-"
	path := filepath.Join(directory, prefix+issueSlug(issue.Title)+".md")
	var existing string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			if !force {
				return "", fmt.Errorf("a draft for issue %d already exists: %s", issue.Number, entry.Name())
			}
			if existing != "" {
				return "", fmt.Errorf("multiple drafts for issue %d exist; resolve them before using --force", issue.Number)
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("refusing to overwrite symlink: %s", entry.Name())
			}
			existing = filepath.Join(directory, entry.Name())
		}
	}
	if existing != "" {
		if err := replacePulledIssue(existing, frontmatter, issue.Body); err != nil {
			return "", err
		}
		return existing, nil
	}
	// #nosec G304 -- path is constructed beneath the selected output directory
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create issue file: %w", err)
	}
	_, writeErr := fmt.Fprintf(file, "---\n%s---\n%s", frontmatter, issue.Body)
	closeErr := file.Close()
	if writeErr != nil {
		return "", fmt.Errorf("write issue file: %w", writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close issue file: %w", closeErr)
	}
	return path, nil
}

func ensurePullDirectory(directory string, defaultDirectory bool) error {
	_, statErr := os.Stat(directory)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect issues directory: %w", statErr)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create issues directory: %w", err)
	}
	if statErr == nil && !defaultDirectory {
		return nil
	}
	ignorePath := filepath.Join(directory, ".gitignore")
	// #nosec G304 -- path is constructed beneath the selected output directory
	ignoreFile, err := os.OpenFile(ignorePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create issues .gitignore: %w", err)
	}
	_, writeErr := ignoreFile.WriteString("*\n!.gitignore\n")
	closeErr := ignoreFile.Close()
	if writeErr != nil {
		return fmt.Errorf("write issues .gitignore: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close issues .gitignore: %w", closeErr)
	}
	return nil
}

func replacePulledIssue(path string, frontmatter []byte, body string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect existing issue file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to overwrite non-regular issue file: %s", path)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".issue-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary issue file: %w", err)
	}
	temporaryPath := file.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	_, writeErr := fmt.Fprintf(file, "---\n%s---\n%s", frontmatter, body)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write issue file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close issue file: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace issue file: %w", err)
	}
	return nil
}

func issueSlug(title string) string {
	var slug strings.Builder
	for _, character := range strings.ToLower(title) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if slug.Len()+len(string(character)) > 100 {
				break
			}
			slug.WriteRune(character)
		} else if unicode.IsSpace(character) || character == '-' || character == '_' {
			if slug.Len() > 0 && !strings.HasSuffix(slug.String(), "_") && slug.Len() < 100 {
				slug.WriteByte('_')
			}
		}
	}
	result := strings.Trim(slug.String(), "_")
	if result == "" {
		return "issue"
	}
	return result
}

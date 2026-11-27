package mkissue

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type pushDraft struct {
	number   int
	metadata *IssueMetadata
	body     string
	source   sourceSnapshot
}

type pushState struct {
	title     string
	body      string
	comments  int
	assignees []string
	labels    []string
	milestone string
	projects  []string
}

type pushPlan struct {
	args []string
	body string
}

type githubRunner func(args []string, stdin string) ([]byte, error)

const (
	pushAPICommand     = "api"
	pushIssueCommand   = "issue"
	pushGraphQLCommand = "graphql"
	pushAssigneeFlag   = "assignee"
	pushPaginate       = "--paginate"
	pushSlurp          = "--slurp"
)

type pushIssueResponse struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      *string `json:"body"`
	Comments  *int    `json:"comments"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	PullRequest json.RawMessage `json:"pull_request"`
}

// Push updates issues from local drafts after preflighting the entire batch.
func Push(numbers []int, directory string, force bool, output io.Writer) error {
	return PushWithOptions(numbers, PushOptions{Directory: directory, Force: force}, output)
}

// PushOptions controls draft selection and cleanup.
type PushOptions struct {
	Directory    string
	Force        bool
	Clean        bool
	CreateLabels bool
}

// PushWithOptions updates issues from local drafts using the requested options.
func PushWithOptions(numbers []int, options PushOptions, output io.Writer) error {
	if len(numbers) == 0 {
		return fmt.Errorf("at least one issue number is required")
	}
	seen := make(map[int]bool)
	unique := make([]int, 0, len(numbers))
	for _, number := range numbers {
		if number <= 0 {
			return fmt.Errorf("issue numbers must be positive integers")
		}
		if !seen[number] {
			seen[number] = true
			unique = append(unique, number)
		}
	}
	rootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("find repository root: %w", err)
	}
	root := strings.TrimSpace(string(rootOutput))
	directory := options.Directory
	if directory == "" {
		directory = filepath.Join(root, "issues")
	} else {
		directory, err = filepath.Abs(directory)
		if err != nil {
			return fmt.Errorf("resolve issue directory: %w", err)
		}
	}
	drafts, err := preparePushDrafts(directory, unique)
	if err != nil {
		return err
	}
	run := newGitHubRunner(root)
	return pushDraftsWithOptions(drafts, options, output, run)
}

func newGitHubRunner(directory string) githubRunner {
	return func(args []string, stdin string) ([]byte, error) {
		// #nosec G204 -- arguments are passed directly to gh without a shell
		command := exec.Command("gh", args...)
		command.Dir = directory
		command.Stdin = strings.NewReader(stdin)
		result, err := command.Output()
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) && len(exitError.Stderr) > 0 {
				return nil, fmt.Errorf("gh: %s: %w", strings.TrimSpace(string(exitError.Stderr)), err)
			}
			return nil, fmt.Errorf("gh: %w", err)
		}
		return result, nil
	}
}

func pushDrafts(drafts []pushDraft, force bool, output io.Writer, run githubRunner) error {
	return pushDraftsWithOptions(drafts, PushOptions{Force: force}, output, run)
}

func pushDraftsWithOptions(drafts []pushDraft, options PushOptions, output io.Writer, run githubRunner) error {
	var failures []error
	for _, draft := range drafts {
		if err := pushOneWithOptions(draft, options.Force, options.CreateLabels, run); err != nil {
			failures = append(failures, fmt.Errorf("issue #%d: %w", draft.number, err))
			continue
		}
		if options.Clean {
			if err := cleanSource(draft.source); err != nil {
				failures = append(failures, fmt.Errorf("issue #%d updated, but source cleanup failed: %w", draft.number, err))
				continue
			}
		}
		if _, err := fmt.Fprintf(output, "Updated issue #%d\n", draft.number); err != nil {
			return fmt.Errorf("report issue #%d: %w", draft.number, err)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("failed to complete %d of %d issues: %w", len(failures), len(drafts), errors.Join(failures...))
	}
	return nil
}

func pushOne(draft pushDraft, force bool, run githubRunner) error {
	return pushOneWithOptions(draft, force, false, run)
}

func pushOneWithOptions(draft pushDraft, force, createLabels bool, run githubRunner) error {
	state, err := fetchPushState(draft.number, run)
	if err != nil {
		return err
	}
	if _, err := pushBodyChanged(draft.body, state.body, state.comments, force); err != nil {
		return err
	}
	metadata := *draft.metadata
	metadata.Assignees = append([]string(nil), draft.metadata.Assignees...)
	for index, assignee := range metadata.Assignees {
		if strings.EqualFold(assignee, "me") {
			user, err := run([]string{pushAPICommand, "user"}, "")
			if err != nil {
				return fmt.Errorf("resolve @me: %w", err)
			}
			var currentUser struct {
				Login string `json:"login"`
			}
			if err := json.Unmarshal(user, &currentUser); err != nil || currentUser.Login == "" {
				return fmt.Errorf("resolve @me: invalid user response")
			}
			metadata.Assignees[index] = currentUser.Login
		}
	}
	draft.metadata = &metadata
	state.projects, err = fetchPushProjects(draft.number, run)
	if err != nil {
		return err
	}
	plan, err := buildPushPlan(draft, state, force)
	if err != nil {
		return err
	}
	if len(plan.args) == 0 {
		return nil
	}
	if err := validateMilestone(draft.metadata.Milestone, state.milestone, run); err != nil {
		return err
	}
	if err := prepareIssueLabels(draft.metadata.Labels, state.labels, createLabels, run); err != nil {
		return err
	}
	args := append([]string{pushIssueCommand, "edit", strconv.Itoa(draft.number)}, plan.args...)
	if _, err := run(args, plan.body); err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return nil
}

func fetchPushState(number int, run githubRunner) (pushState, error) {
	endpoint := "repos/{owner}/{repo}/issues/" + strconv.Itoa(number)
	data, err := run([]string{pushAPICommand, endpoint}, "")
	if err != nil {
		return pushState{}, fmt.Errorf("fetch issue: %w", err)
	}
	var issue pushIssueResponse
	if err := json.Unmarshal(data, &issue); err != nil {
		return pushState{}, fmt.Errorf("decode issue: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return pushState{}, fmt.Errorf("decode issue fields: %w", err)
	}
	_, hasBody := fields["body"]
	_, hasMilestone := fields["milestone"]
	if issue.Number != number || issue.Title == "" || issue.Comments == nil || *issue.Comments < 0 || issue.Assignees == nil || issue.Labels == nil || !hasBody || !hasMilestone {
		return pushState{}, fmt.Errorf("issue response is missing required fields")
	}
	if len(issue.PullRequest) > 0 && string(issue.PullRequest) != "null" {
		return pushState{}, fmt.Errorf("number refers to a pull request, not an issue")
	}
	state := pushState{title: issue.Title, comments: *issue.Comments}
	if issue.Body != nil {
		state.body = *issue.Body
	}
	for _, assignee := range issue.Assignees {
		if assignee.Login == "" {
			return pushState{}, fmt.Errorf("issue response contains an invalid assignee")
		}
		state.assignees = append(state.assignees, assignee.Login)
	}
	for _, label := range issue.Labels {
		if label.Name == "" {
			return pushState{}, fmt.Errorf("issue response contains an invalid label")
		}
		state.labels = append(state.labels, label.Name)
	}
	if issue.Milestone != nil {
		if issue.Milestone.Title == "" {
			return pushState{}, fmt.Errorf("issue response contains an invalid milestone")
		}
		state.milestone = issue.Milestone.Title
	}
	return state, nil
}

const pushProjectsQuery = `query($owner:String!,$repo:String!,$number:Int!,$endCursor:String){repository(owner:$owner,name:$repo){issue(number:$number){projectItems(first:100,after:$endCursor){nodes{project{id title}} pageInfo{hasNextPage endCursor}}}}}`

func fetchPushProjects(number int, run githubRunner) ([]string, error) {
	data, err := run([]string{pushAPICommand, pushGraphQLCommand, pushPaginate, pushSlurp, "-F", "owner={owner}", "-F", "repo={repo}", "-F", "number=" + strconv.Itoa(number), "-f", "query=" + pushProjectsQuery}, "")
	if err != nil {
		return nil, fmt.Errorf("fetch issue projects: %w", err)
	}
	var pages []struct {
		Errors []json.RawMessage `json:"errors"`
		Data   *struct {
			Repository *struct {
				Issue *struct {
					ProjectItems *struct {
						Nodes []struct {
							Project *struct {
								ID    string `json:"id"`
								Title string `json:"title"`
							} `json:"project"`
						} `json:"nodes"`
						PageInfo *struct {
							HasNextPage *bool `json:"hasNextPage"`
						} `json:"pageInfo"`
					} `json:"projectItems"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &pages); err != nil || len(pages) == 0 {
		return nil, fmt.Errorf("decode issue projects: invalid response")
	}
	var projects []string
	seen := make(map[string]string)
	for index, page := range pages {
		if len(page.Errors) > 0 {
			return nil, fmt.Errorf("decode issue projects: GraphQL response contains errors")
		}
		if page.Data == nil || page.Data.Repository == nil || page.Data.Repository.Issue == nil || page.Data.Repository.Issue.ProjectItems == nil || page.Data.Repository.Issue.ProjectItems.Nodes == nil || page.Data.Repository.Issue.ProjectItems.PageInfo == nil {
			return nil, fmt.Errorf("decode issue projects: incomplete response")
		}
		if page.Data.Repository.Issue.ProjectItems.PageInfo.HasNextPage == nil || *page.Data.Repository.Issue.ProjectItems.PageInfo.HasNextPage != (index < len(pages)-1) {
			return nil, fmt.Errorf("decode issue projects: incomplete pagination")
		}
		for _, item := range page.Data.Repository.Issue.ProjectItems.Nodes {
			if item.Project == nil || item.Project.ID == "" || item.Project.Title == "" {
				return nil, fmt.Errorf("decode issue projects: missing project identity")
			}
			if id, exists := seen[item.Project.Title]; exists && id != item.Project.ID {
				return nil, fmt.Errorf("multiple current projects share the title %q", item.Project.Title)
			} else if !exists {
				seen[item.Project.Title] = item.Project.ID
				projects = append(projects, item.Project.Title)
			}
		}
	}
	return projects, nil
}

func prepareIssueLabels(labels []Label, current []string, createLabels bool, run githubRunner) error {
	desired := make([]string, 0, len(labels))
	for _, label := range labels {
		desired = append(desired, label.Name)
	}
	if len(setDifference(desired, current, true)) == 0 {
		return nil
	}
	data, err := run([]string{pushAPICommand, pushPaginate, pushSlurp, "repos/{owner}/{repo}/labels?per_page=100"}, "")
	if err != nil {
		return fmt.Errorf("fetch repository labels: %w", err)
	}
	var pages [][]struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pages); err != nil || pages == nil {
		return fmt.Errorf("decode repository labels: invalid response")
	}
	existing := make(map[string]bool)
	for _, page := range pages {
		if page == nil {
			return fmt.Errorf("decode repository labels: incomplete page")
		}
		for _, label := range page {
			if label.Name == "" {
				return fmt.Errorf("decode repository labels: missing label name")
			}
			existing[strings.ToLower(label.Name)] = true
		}
	}
	var missing []Label
	for _, label := range labels {
		key := strings.ToLower(label.Name)
		if existing[key] {
			continue
		}
		if !createLabels {
			return fmt.Errorf("label %q does not exist; use --create-labels to create it", label.Name)
		}
		missing = append(missing, label)
		existing[key] = true
	}
	for _, label := range missing {
		payload := struct {
			Name        string `json:"name"`
			Color       string `json:"color,omitempty"`
			Description string `json:"description,omitempty"`
		}{Name: label.Name, Color: label.Color, Description: label.Desc}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode label %q: %w", label.Name, err)
		}
		if _, err := run([]string{pushAPICommand, "--method", "POST", "repos/{owner}/{repo}/labels", "--input", "-"}, string(data)); err != nil {
			return fmt.Errorf("create label %q: %w", label.Name, err)
		}
	}
	return nil
}

func validateMilestone(desired, current string, run githubRunner) error {
	if desired == "" || desired == current {
		return nil
	}
	data, err := run([]string{pushAPICommand, pushPaginate, pushSlurp, "repos/{owner}/{repo}/milestones?state=all&per_page=100"}, "")
	if err != nil {
		return fmt.Errorf("fetch milestones: %w", err)
	}
	var pages [][]struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &pages); err != nil || pages == nil {
		return fmt.Errorf("decode milestones: invalid response")
	}
	for _, page := range pages {
		if page == nil {
			return fmt.Errorf("decode milestones: incomplete page")
		}
		for _, milestone := range page {
			if milestone.Title == "" {
				return fmt.Errorf("decode milestones: missing title")
			}
			if milestone.Title == desired {
				return nil
			}
		}
	}
	return fmt.Errorf("milestone %q does not exist", desired)
}

func buildPushPlan(draft pushDraft, current pushState, force bool) (pushPlan, error) {
	changed, err := pushBodyChanged(draft.body, current.body, current.comments, force)
	if err != nil {
		return pushPlan{}, err
	}
	plan := pushPlan{}
	if draft.metadata.Title != current.title {
		plan.args = append(plan.args, "--title", draft.metadata.Title)
	}
	if changed {
		plan.args = append(plan.args, "--body-file", "-")
		plan.body = draft.body
	}
	if draft.metadata.Milestone != current.milestone {
		if draft.metadata.Milestone == "" {
			plan.args = append(plan.args, "--remove-milestone")
		} else {
			plan.args = append(plan.args, "--milestone", draft.metadata.Milestone)
		}
	}
	labels := make([]string, 0, len(draft.metadata.Labels))
	for _, label := range draft.metadata.Labels {
		labels = append(labels, label.Name)
	}
	fields := []struct {
		flag       string
		current    []string
		desired    []string
		ignoreCase bool
	}{
		{flag: pushAssigneeFlag, current: current.assignees, desired: draft.metadata.Assignees, ignoreCase: true},
		{flag: "label", current: current.labels, desired: labels, ignoreCase: true},
		{flag: "project", current: current.projects, desired: draft.metadata.Projects},
	}
	for _, field := range fields {
		for _, value := range setDifference(field.desired, field.current, field.ignoreCase) {
			argument := value
			if field.flag == pushAssigneeFlag && strings.EqualFold(value, "copilot") {
				argument = "@copilot"
			}
			encoded, err := csvArgument(argument)
			if err != nil {
				return pushPlan{}, err
			}
			plan.args = append(plan.args, "--add-"+field.flag, encoded)
		}
		for _, value := range setDifference(field.current, field.desired, field.ignoreCase) {
			argument := value
			if field.flag == pushAssigneeFlag && strings.EqualFold(value, "copilot") {
				argument = "@copilot"
			}
			encoded, err := csvArgument(argument)
			if err != nil {
				return pushPlan{}, err
			}
			plan.args = append(plan.args, "--remove-"+field.flag, encoded)
		}
	}
	return plan, nil
}

func setDifference(left, right []string, ignoreCase bool) []string {
	seen := make(map[string]bool)
	for _, value := range right {
		key := value
		if ignoreCase {
			key = strings.ToLower(key)
		}
		seen[key] = true
	}
	var result []string
	for _, value := range left {
		key := value
		if ignoreCase {
			key = strings.ToLower(key)
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, value)
		}
	}
	return result
}

func csvArgument(value string) (string, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{value}); err != nil {
		return "", fmt.Errorf("encode edit argument: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("encode edit argument: %w", err)
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

func preparePushDrafts(directory string, numbers []int) ([]pushDraft, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read issue directory: %w", err)
	}
	drafts := make([]pushDraft, 0, len(numbers))
	for _, number := range numbers {
		prefix := strconv.Itoa(number) + "-"
		var match string
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			if match != "" {
				return nil, fmt.Errorf("multiple drafts for issue %d in %s", number, directory)
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("refusing to read symlink for issue %d: %s", number, entry.Name())
			}
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("inspect issue %d draft: %w", number, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("issue %d draft is not a regular file: %s", number, entry.Name())
			}
			match = filepath.Join(directory, entry.Name())
		}
		if match == "" {
			return nil, fmt.Errorf("no draft for issue %d in %s", number, directory)
		}
		content, snapshot, err := readLocalSource(match)
		if err != nil {
			return nil, fmt.Errorf("read issue %d draft: %w", number, err)
		}
		metadata, body, err := parseDraftFile(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse issue %d draft: %w", number, err)
		}
		drafts = append(drafts, pushDraft{number: number, metadata: metadata, body: body, source: snapshot})
	}
	return drafts, nil
}

func parseDraftFile(content string) (*IssueMetadata, string, error) {
	delimiters := frontmatterDelimiter.FindAllStringIndex(content, 2)
	if len(delimiters) < 2 || delimiters[0][0] != 0 {
		return nil, "", fmt.Errorf("invalid format: frontmatter not found")
	}
	frontmatter := content[delimiters[0][1]:delimiters[1][0]]
	var metadata IssueMetadata
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, "", fmt.Errorf("invalid issue frontmatter: %w", err)
	}
	if strings.TrimSpace(metadata.Title) == "" {
		return nil, "", fmt.Errorf("'title' is required in frontmatter")
	}
	for index, assignee := range metadata.Assignees {
		metadata.Assignees[index] = strings.TrimPrefix(assignee, "@")
		if strings.TrimSpace(metadata.Assignees[index]) == "" {
			return nil, "", fmt.Errorf("assignee cannot be empty")
		}
	}
	for _, label := range metadata.Labels {
		if strings.TrimSpace(label.Name) == "" {
			return nil, "", fmt.Errorf("label name cannot be empty")
		}
	}
	for _, project := range metadata.Projects {
		if strings.TrimSpace(project) == "" {
			return nil, "", fmt.Errorf("project title cannot be empty")
		}
	}
	body := strings.TrimPrefix(content[delimiters[1][1]:], "\n")
	return &metadata, body, nil
}

func pushBodyChanged(local, remote string, commentCount int, force bool) (bool, error) {
	changed := local != remote
	if changed && commentCount > 0 && !force {
		return false, fmt.Errorf("issue has comments; use --force to update its description")
	}
	return changed, nil
}

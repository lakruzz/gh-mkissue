package mkissue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const fakeIssue = `{"number":7,"title":"Old","body":"old body\n","comments":2,"assignees":[{"login":"alice"},{"login":"Copilot"}],"labels":[{"name":"defect"},{"name":"old"}],"milestone":{"title":"Old milestone"}}`
const fakeProjects = `[{"data":{"repository":{"issue":{"projectItems":{"nodes":[{"project":{"id":"project-id","title":"Old project"}}],"pageInfo":{"hasNextPage":false}}}}}}]`
const oldBody = "old body\n"
const oldTitle = "Old"
const oldValue = "old"
const newTitle = "New"
const firstDraft = "7-original.md"
const firstFrontmatter = "---\ntitle: First\n---\n"

func TestPushPreflightAndCommandIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh executable uses a POSIX shell")
	}
	root := t.TempDir()
	// #nosec G204 -- root is the test's temporary directory
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "gh.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$PUSH_TEST_LOG\"\n" +
		"case \"$1/$2\" in\n" +
		"api/graphql) printf '%s' '" + fakeProjects + "' ;;\n" +
		"api/repos/*) printf '%s' '" + fakeIssue + "' ;;\n" +
		"issue/edit) printf '%s' 'updated' ;;\n" +
		"*) exit 1 ;;\nesac\n"
	// #nosec G306 -- the isolated fake gh helper must be executable
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PUSH_TEST_LOG", logPath)
	directory := filepath.Join(root, "drafts")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{firstDraft, "8-first.md", "8-second.md"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("---\ntitle: New\n---\n"+oldBody), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Push([]int{7, 8}, directory, false, io.Discard); err == nil {
		t.Fatal("expected ambiguous drafts to abort the batch")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("GitHub command ran before batch preflight: %v", err)
	}
	if err := Push([]int{7}, directory, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- log path is inside the test's temporary directory
	log, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(log), "issue edit 7") || strings.Contains(string(log), "--body-file") {
		t.Errorf("unexpected fake GitHub commands: %q, error = %v", log, err)
	}
}

func TestPushOneGuardAndUpdate(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		force        bool
		wantErr      bool
		wantProjects bool
		wantEdit     bool
	}{
		{name: "blocked", body: "changed\n", wantErr: true},
		{name: "metadata only", body: oldBody, wantProjects: true, wantEdit: true},
		{name: "forced", body: "changed\n", force: true, wantProjects: true, wantEdit: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			var editArgs []string
			var editBody string
			run := func(args []string, stdin string) ([]byte, error) {
				calls = append(calls, strings.Join(args, " "))
				switch {
				case len(args) > 1 && args[0] == pushAPICommand && strings.HasPrefix(args[1], "repos/"):
					return []byte(fakeIssue), nil
				case len(args) > 1 && args[0] == pushAPICommand && args[1] == pushGraphQLCommand:
					return []byte(fakeProjects), nil
				case len(args) > 1 && args[0] == pushIssueCommand && args[1] == "edit":
					editArgs = append([]string(nil), args...)
					editBody = stdin
					return nil, nil
				default:
					return nil, fmt.Errorf("unexpected command: %v", args)
				}
			}
			draft := pushDraft{number: 7, metadata: &IssueMetadata{Title: newTitle}, body: test.body}
			err := pushOne(draft, test.force, run)
			if (err != nil) != test.wantErr || (len(calls) > 1) != test.wantProjects || (editArgs != nil) != test.wantEdit {
				t.Fatalf("calls = %v, error = %v", calls, err)
			}
			if test.wantEdit {
				joined := strings.Join(editArgs, " ")
				for _, flag := range []string{"--remove-assignee alice", "--remove-assignee @copilot", "--remove-label defect", "--remove-label old", "--remove-milestone", "--remove-project Old project"} {
					if !strings.Contains(joined, flag) {
						t.Errorf("edit args %v missing %q", editArgs, flag)
					}
				}
				if strings.Contains(joined, "--body-file") != test.force || (test.force && editBody != test.body) {
					t.Errorf("edit args = %v, body = %q", editArgs, editBody)
				}
			}
		})
	}
}

func TestFetchPushStateRejectsIncompleteResponses(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "missing body", data: `{"number":7,"title":"Issue","comments":0,"assignees":[],"labels":[],"milestone":null}`},
		{name: "missing comments", data: `{"number":7,"title":"Issue","body":"","assignees":[],"labels":[],"milestone":null}`},
		{name: "missing milestone", data: `{"number":7,"title":"Issue","body":"","comments":0,"assignees":[],"labels":[]}`},
		{name: "pull request", data: `{"number":7,"title":"Issue","body":"","comments":0,"assignees":[],"labels":[],"milestone":null,"pull_request":{}}`},
		{name: "invalid assignee", data: `{"number":7,"title":"Issue","body":"","comments":0,"assignees":[{}],"labels":[],"milestone":null}`},
		{name: "invalid label", data: `{"number":7,"title":"Issue","body":"","comments":0,"assignees":[],"labels":[{}],"milestone":null}`},
		{name: "invalid milestone", data: `{"number":7,"title":"Issue","body":"","comments":0,"assignees":[],"labels":[],"milestone":{}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fetchPushState(7, func(_ []string, _ string) ([]byte, error) { return []byte(test.data), nil })
			if err == nil {
				t.Fatal("expected an error for an unsafe remote response")
			}
		})
	}
}

func TestPushOneResolvesSelfAssignee(t *testing.T) {
	var editArgs []string
	run := func(args []string, _ string) ([]byte, error) {
		switch {
		case args[0] == pushAPICommand && args[1] == "user":
			return []byte(`{"login":"alice"}`), nil
		case args[0] == pushAPICommand && args[1] == pushGraphQLCommand:
			return []byte(fakeProjects), nil
		case args[0] == pushAPICommand:
			return []byte(fakeIssue), nil
		case args[0] == pushIssueCommand:
			editArgs = append([]string(nil), args...)
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected command: %v", args)
		}
	}
	draft := pushDraft{number: 7, metadata: &IssueMetadata{Title: oldTitle, Assignees: []string{"me"}}, body: oldBody}
	if err := pushOne(draft, false, run); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(editArgs, " "), "--remove-assignee alice") || !strings.Contains(strings.Join(editArgs, " "), "--remove-assignee @copilot") {
		t.Errorf("unexpected self-assignee update: %v", editArgs)
	}
}

func TestFetchPushProjects(t *testing.T) {
	page := func(id, title string, hasNext bool) string {
		return fmt.Sprintf(`{"data":{"repository":{"issue":{"projectItems":{"nodes":[{"project":{"id":%q,"title":%q}}],"pageInfo":{"hasNextPage":%t}}}}}}`, id, title, hasNext)
	}
	tests := []struct {
		name    string
		data    string
		want    []string
		wantErr bool
	}{
		{name: "pages", data: "[" + page("one", "A", true) + "," + page("two", "B", false) + "]", want: []string{"A", "B"}},
		{name: "ambiguous title", data: "[" + page("one", "A", true) + "," + page("two", "A", false) + "]", wantErr: true},
		{name: "truncated", data: "[" + page("one", "A", true) + "]", wantErr: true},
		{name: "incomplete", data: `[{"data":{"repository":null}}]`, wantErr: true},
		{name: "missing page flag", data: `[{"data":{"repository":{"issue":{"projectItems":{"nodes":[],"pageInfo":{}}}}}}]`, wantErr: true},
		{name: "graphql error", data: `[{"errors":[{"message":"access denied"}]}]`, wantErr: true},
		{name: "empty response", data: `[]`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := func(args []string, _ string) ([]byte, error) {
				if !strings.Contains(strings.Join(args, " "), "--paginate --slurp") {
					t.Errorf("project query does not paginate: %v", args)
				}
				return []byte(test.data), nil
			}
			got, err := fetchPushProjects(7, run)
			if (err != nil) != test.wantErr || !reflect.DeepEqual(got, test.want) {
				t.Errorf("projects = %v, error = %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestPreparePushLabels(t *testing.T) {
	var created []map[string]string
	run := func(args []string, stdin string) ([]byte, error) {
		if args[0] != pushAPICommand {
			return nil, fmt.Errorf("unexpected command: %v", args)
		}
		if strings.Contains(strings.Join(args, " "), "--method POST") {
			var label map[string]string
			if err := json.Unmarshal([]byte(stdin), &label); err != nil {
				return nil, err
			}
			created = append(created, label)
			return []byte(`{}`), nil
		}
		return []byte(`[[{"name":"first"}],[{"name":"existing"}]]`), nil
	}
	labels := []Label{{Name: "existing"}, {Name: "new, label", Desc: "A description"}}
	if err := prepareIssueLabels(labels, nil, true, run); err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0]["name"] != "new, label" || created[0]["description"] != "A description" {
		t.Errorf("created labels = %v", created)
	}
	created = nil
	if err := prepareIssueLabels([]Label{{Name: "create", Color: "123456"}, {Name: "missing"}}, nil, false, run); err == nil || len(created) != 0 {
		t.Errorf("expected opt-in error before creation; created = %v, error = %v", created, err)
	}
	if err := prepareIssueLabels([]Label{{Name: "plain"}}, nil, true, run); err != nil || len(created) != 1 || created[0]["name"] != "plain" {
		t.Errorf("name-only label with opt-in: created = %v, error = %v", created, err)
	}
}

func TestValidateMilestone(t *testing.T) {
	const release = "release"
	tests := []struct {
		name    string
		desired string
		current string
		data    string
		wantErr bool
		calls   int
	}{
		{name: "empty", calls: 0},
		{name: "unchanged", desired: release, current: release, calls: 0},
		{name: "later page", desired: release, data: `[[{"title":"first"}],[{"title":"release"}]]`, calls: 1},
		{name: "missing milestone", desired: "not-found", data: `[[{"title":"release"}]]`, calls: 1, wantErr: true},
		{name: "incomplete response", desired: release, data: `[null]`, calls: 1, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			run := func(args []string, _ string) ([]byte, error) {
				calls++
				if !strings.Contains(strings.Join(args, " "), "--paginate --slurp") {
					t.Errorf("milestone query does not paginate: %v", args)
				}
				return []byte(test.data), nil
			}
			err := validateMilestone(test.desired, test.current, run)
			if (err != nil) != test.wantErr || calls != test.calls {
				t.Errorf("error = %v, calls = %d; want error %v, calls %d", err, calls, test.wantErr, test.calls)
			}
		})
	}
}

func TestPushValidatesMilestoneBeforeCreatingLabels(t *testing.T) {
	var calls []string
	run := func(args []string, _ string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		if args[0] == pushAPICommand && args[1] == pushGraphQLCommand {
			return []byte(fakeProjects), nil
		}
		if args[0] == pushAPICommand && strings.Contains(strings.Join(args, " "), "/milestones?") {
			return []byte(`[[]]`), nil
		}
		if args[0] == pushAPICommand && strings.HasPrefix(args[1], "repos/") {
			return []byte(fakeIssue), nil
		}
		return nil, fmt.Errorf("unexpected command: %v", args)
	}
	draft := pushDraft{number: 7, metadata: &IssueMetadata{Title: newTitle, Milestone: "not-found", Labels: []Label{{Name: "new label"}}}, body: oldBody}
	err := pushOneWithOptions(draft, false, true, run)
	if err == nil || !strings.Contains(err.Error(), "milestone") || len(calls) != 3 {
		t.Errorf("calls = %v, error = %v", calls, err)
	}
}
func TestPushDraftsContinuesAfterErrors(t *testing.T) {
	var output bytes.Buffer
	drafts := []pushDraft{
		{number: 7, metadata: &IssueMetadata{Title: oldTitle}, body: "blocked"},
		{number: 8, metadata: &IssueMetadata{Title: oldTitle}, body: oldBody},
	}
	run := func(args []string, _ string) ([]byte, error) {
		if args[0] == pushAPICommand && args[1] == pushGraphQLCommand {
			return []byte(fakeProjects), nil
		}
		if args[0] == pushAPICommand && strings.HasSuffix(args[1], "/issues/8") {
			return []byte(strings.Replace(fakeIssue, `"number":7`, `"number":8`, 1)), nil
		}
		if args[0] == pushAPICommand {
			return []byte(fakeIssue), nil
		}
		return nil, nil
	}
	err := pushDrafts(drafts, false, &output, run)
	if err == nil || !strings.Contains(err.Error(), "issue #7") || output.String() != "Updated issue #8\n" {
		t.Errorf("output = %q, error = %v", output.String(), err)
	}
}

func TestBuildPushPlan(t *testing.T) {
	current := pushState{title: oldTitle, body: oldBody, comments: 2, assignees: []string{"alice", "Copilot"}, labels: []string{"defect", oldValue}, milestone: "Old milestone", projects: []string{"Old project"}}
	draft := pushDraft{metadata: &IssueMetadata{Title: newTitle, Assignees: []string{"ALICE"}, Labels: []Label{{Name: "defect"}, {Name: "help, wanted"}}, Projects: []string{"Roadmap, Q1"}}, body: "new body\n"}
	if _, err := buildPushPlan(draft, current, false); err == nil {
		t.Fatal("expected the comment guard to reject a body change")
	}
	plan, err := buildPushPlan(draft, current, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--title", newTitle, "--body-file", "-", "--remove-milestone", "--remove-assignee", "@copilot", "--add-label", `"help, wanted"`, "--remove-label", "old", "--add-project", `"Roadmap, Q1"`, "--remove-project", "Old project"}
	if !reflect.DeepEqual(plan.args, want) || plan.body != draft.body {
		t.Errorf("plan = %+v, want args %v and body %q", plan, want, draft.body)
	}
	draft.body = current.body
	if _, err := buildPushPlan(draft, current, false); err != nil {
		t.Fatalf("metadata-only changes should be allowed with comments: %v", err)
	}
}

func TestPreparePushDrafts(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		numbers []int
		wantErr bool
	}{
		{name: "multiple drafts", files: map[string]string{firstDraft: firstFrontmatter + "Body", "8-next.md": "---\ntitle: Next\n---\n"}, numbers: []int{7, 8}},
		{name: "missing", files: map[string]string{firstDraft: firstFrontmatter}, numbers: []int{7, 8}, wantErr: true},
		{name: "ambiguous", files: map[string]string{firstDraft: firstFrontmatter, "7-other.md": "---\ntitle: Other\n---\n"}, numbers: []int{7}, wantErr: true},
		{name: "invalid later draft", files: map[string]string{firstDraft: firstFrontmatter, "8-next.md": "invalid"}, numbers: []int{7, 8}, wantErr: true},
		{name: "number prefix", files: map[string]string{"70-other.md": "---\ntitle: Other\n---\n"}, numbers: []int{7}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			for name, content := range test.files {
				if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			drafts, err := preparePushDrafts(directory, test.numbers)
			if (err != nil) != test.wantErr || (!test.wantErr && len(drafts) != len(test.numbers)) {
				t.Errorf("preparePushDrafts() = %v, %v; want error %v", drafts, err, test.wantErr)
			}
		})
	}
}

func TestPreparePushRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "source.md")
	if err := os.WriteFile(path, []byte("---\ntitle: Issue\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, filepath.Join(directory, "7-issue.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := preparePushDrafts(directory, []int{7}); err == nil {
		t.Fatal("expected symlink draft to be rejected")
	}
}

func TestParsePushFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		body    string
		wantErr bool
	}{
		{name: "body whitespace", content: "---\ntitle: Issue\n---\n\n body \n\n", body: "\n body \n\n"},
		{name: "empty body", content: "---\ntitle: Issue\n---\n", body: ""},
		{name: "windows newlines", content: "---\r\ntitle: Issue\r\n---\r\nbody\r\n", body: "body\r\n"},
		{name: "unknown field", content: "---\ntitle: Issue\nlabel: bug\n---\n", wantErr: true},
		{name: "invalid yaml", content: "---\ntitle: [\n---\n", wantErr: true},
		{name: "empty title", content: "---\ntitle: ''\n---\n", wantErr: true},
		{name: "empty label", content: "---\ntitle: Issue\nlabels:\n  - name: ''\n---\n", wantErr: true},
		{name: "empty assignee", content: "---\ntitle: Issue\nassign: ['']\n---\n", wantErr: true},
		{name: "empty project", content: "---\ntitle: Issue\nprojects: ['']\n---\n", wantErr: true},
		{name: "missing frontmatter", content: "Issue body", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, body, err := parseDraftFile(test.content)
			if (err != nil) != test.wantErr || (!test.wantErr && body != test.body) {
				t.Errorf("parsePushFile() body = %q, error = %v; want %q, error %v", body, err, test.body, test.wantErr)
			}
		})
	}
}

func TestPushBodyChanged(t *testing.T) {
	const updated = "new"
	tests := []struct {
		name     string
		local    string
		remote   string
		comments int
		force    bool
		changed  bool
		wantErr  bool
	}{
		{name: "unchanged with comments", local: "same", remote: "same", comments: 2},
		{name: "changed without comments", local: updated, remote: oldValue, changed: true},
		{name: "blocked with comments", local: updated, remote: oldValue, comments: 1, wantErr: true},
		{name: "forced with comments", local: updated, remote: oldValue, comments: 1, force: true, changed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed, err := pushBodyChanged(test.local, test.remote, test.comments, test.force)
			if changed != test.changed || (err != nil) != test.wantErr {
				t.Errorf("pushBodyChanged() = %v, %v; want %v, error %v", changed, err, test.changed, test.wantErr)
			}
		})
	}
}

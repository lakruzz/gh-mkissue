package mkissue

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const exampleTitle = "Example"
const expectedIgnore = "*\n!.gitignore\n"

func TestParseOpenIssuePages(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []int
		wantErr bool
	}{
		{name: "multiple pages and pull requests", data: `[[{"number":4},{"number":5,"pull_request":{}}],[{"number":89,"pull_request":null},{"number":4}]]`, want: []int{4, 89}},
		{name: "empty", data: `[[]]`},
		{name: "invalid response", data: `{}`, wantErr: true},
		{name: "null response", data: `null`, wantErr: true},
		{name: "missing number", data: `[[{}]]`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseOpenIssuePages([]byte(test.data))
			if (err != nil) != test.wantErr || !reflect.DeepEqual(got, test.want) {
				t.Errorf("parseOpenIssuePages() = %v, %v; want %v, error %v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestSavePulledIssue(t *testing.T) {
	data := `{"number":7,"title":"A \"quoted\" --- issue #1","body":"First line\n---\nSecond line\n","assignees":[{"login":"alice"},{"login":"bob"}],"labels":[{"name":"help, wanted"},{"name":"urgent request"}],"milestone":{"title":"Release: \"one\""},"projectItems":[{"title":"Board, \"one\""},{"title":"Other board"}]}`
	var issue pulledIssue
	if err := json.Unmarshal([]byte(data), &issue); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path, err := savePulledIssue(root, issue)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "7-a_quoted_issue_1.md" {
		t.Errorf("unexpected filename: %s", path)
	}
	// #nosec G304 -- path is returned from the test's temporary directory
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata, body, err := parseIssueFile(string(content))
	if err != nil {
		t.Fatal(err)
	}
	want := &IssueMetadata{
		Title:     issue.Title,
		Assignees: []string{"alice", "bob"},
		Labels:    []Label{{Name: "help, wanted"}, {Name: "urgent request"}},
		Milestone: `Release: "one"`,
		Projects:  []string{`Board, "one"`, "Other board"},
	}
	if !reflect.DeepEqual(metadata, want) || body != strings.TrimSpace(issue.Body) {
		t.Errorf("metadata = %+v, body = %q; want %+v, %q", metadata, body, want, issue.Body)
	}
	if !strings.HasSuffix(string(content), issue.Body) {
		t.Error("issue body was not saved verbatim")
	}
	// #nosec G304 -- path is inside the test's temporary directory
	ignore, err := os.ReadFile(filepath.Join(root, "issues", ".gitignore"))
	if err != nil || string(ignore) != expectedIgnore {
		t.Errorf("gitignore = %q, error = %v", ignore, err)
	}
	issue.Title = "A different title"
	if _, err := savePulledIssue(root, issue); err == nil {
		t.Fatal("expected an error for an existing issue draft")
	}
	// #nosec G304 -- path is returned from the test's temporary directory
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(content) {
		t.Errorf("existing draft changed, error = %v", err)
	}
}

func TestPullPreservesExistingIgnore(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "issues")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	ignorePath := filepath.Join(directory, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte("custom\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := savePulledIssue(root, pulledIssue{Number: 1, Title: exampleTitle}); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- path is inside the test's temporary directory
	content, err := os.ReadFile(ignorePath)
	if err != nil || string(content) != "custom\n" {
		t.Errorf("existing gitignore = %q, error = %v", content, err)
	}
}

func TestPullCustomDirectoryAndForce(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "nested", "drafts")
	issue := pulledIssue{Number: 7, Title: "Old title", Body: "Original"}
	path, err := savePulledIssueTo(directory, issue, false, false)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- path is inside the test's temporary directory
	ignore, err := os.ReadFile(filepath.Join(directory, ".gitignore"))
	if err != nil || string(ignore) != expectedIgnore {
		t.Fatalf("gitignore = %q, error = %v", ignore, err)
	}
	issue.Title = "New title"
	issue.Body = "Updated"
	if _, err := savePulledIssueTo(directory, issue, false, false); err == nil {
		t.Fatal("expected an error without force")
	}
	replaced, err := savePulledIssueTo(directory, issue, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if replaced != path {
		t.Fatalf("replacement path = %q, want %q", replaced, path)
	}
	// #nosec G304 -- path is inside the test's temporary directory
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata, body, err := parseIssueFile(string(content))
	if err != nil || metadata.Title != issue.Title || body != issue.Body {
		t.Errorf("metadata = %+v, body = %q, error = %v", metadata, body, err)
	}
}

func TestPullForceRejectsMultipleDrafts(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"7-first.md", "7-second.md"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := savePulledIssueTo(directory, pulledIssue{Number: 7, Title: exampleTitle}, true, false); err == nil {
		t.Fatal("expected an error for multiple matching drafts")
	}
	for _, name := range []string{"7-first.md", "7-second.md"} {
		// #nosec G304 -- path is inside the test's temporary directory
		content, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(content) != name {
			t.Errorf("draft %s was changed: %q, error = %v", name, content, err)
		}
	}
}

func TestPullForceRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "other.md")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory, "7-example.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := savePulledIssueTo(directory, pulledIssue{Number: 7, Title: exampleTitle}, true, false); err == nil {
		t.Fatal("expected an error for a symlink draft")
	}
	// #nosec G304 -- path is inside the test's temporary directory
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "keep" {
		t.Errorf("symlink target changed: %q, error = %v", content, err)
	}
}

func TestPullExistingCustomDirectoryDoesNotCreateIgnore(t *testing.T) {
	directory := t.TempDir()
	if _, err := savePulledIssueTo(directory, pulledIssue{Number: 7, Title: exampleTitle}, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("expected no new .gitignore in existing directory, got %v", err)
	}
}

func TestPullExistingDirectoryWithoutIgnore(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "issues"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := savePulledIssue(root, pulledIssue{Number: 1, Title: exampleTitle}); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- path is inside the test's temporary directory
	content, err := os.ReadFile(filepath.Join(root, "issues", ".gitignore"))
	if err != nil || string(content) != expectedIgnore {
		t.Errorf("gitignore = %q, error = %v", content, err)
	}
}

func TestPullIgnoreRules(t *testing.T) {
	root := t.TempDir()
	// #nosec G204 -- root is the test's temporary directory
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	path, err := savePulledIssue(root, pulledIssue{Number: 1, Title: exampleTitle})
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G204 -- path is inside the test's temporary directory
	command := exec.Command("git", "check-ignore", "-q", path)
	command.Dir = root
	if err := command.Run(); err != nil {
		t.Errorf("issue file is not ignored: %v", err)
	}
	// #nosec G204 -- path is inside the test's temporary directory
	command = exec.Command("git", "check-ignore", "-q", filepath.Join(root, "issues", ".gitignore"))
	command.Dir = root
	if err := command.Run(); err == nil {
		t.Error("issues/.gitignore should not be ignored")
	}
}

func TestIssueSlug(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{title: " Fix / Broken - Stuff! ", want: "fix_broken_stuff"},
		{title: "UPPER___lower", want: "upper_lower"},
		{title: "Æble & Øl", want: "æble_øl"},
		{title: "!!!", want: "issue"},
		{title: strings.Repeat("a", 120), want: strings.Repeat("a", 100)},
	}
	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			if got := issueSlug(test.title); got != test.want {
				t.Errorf("issueSlug() = %q, want %q", got, test.want)
			}
		})
	}
}

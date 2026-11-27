package mkissue

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const testWindows = "windows"
const sourceTestFile = "issue.md"
const emptyTestPages = "[[]]"

func TestCleanSource(t *testing.T) {
	const editChange = "edit"
	const removeChange = "remove"
	tests := []struct {
		name    string
		change  string
		wantErr bool
	}{
		{name: "unchanged"},
		{name: "edited", change: editChange, wantErr: true},
		{name: "replaced", change: "replace", wantErr: true},
		{name: "removed", change: removeChange, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "draft.md")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			_, snapshot, err := readLocalSource(path)
			if err != nil {
				t.Fatal(err)
			}
			switch test.change {
			case editChange:
				err = os.WriteFile(path, []byte("changed!"), 0600)
			case "replace":
				replacement := filepath.Join(filepath.Dir(path), "replacement")
				if err = os.WriteFile(replacement, []byte("original"), 0600); err == nil {
					err = os.Rename(replacement, path)
				}
			case removeChange:
				err = os.Remove(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = cleanSource(snapshot)
			if (err != nil) != test.wantErr {
				t.Errorf("cleanSource() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.change != removeChange {
				_, statErr := os.Stat(path)
				if (statErr == nil) != test.wantErr {
					t.Errorf("source existence error = %v, want retained %v", statErr, test.wantErr)
				}
			}
		})
	}
}

func TestCleanSourceRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "source.md")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link.md")
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := readLocalSource(link); err == nil {
		t.Fatal("expected symlink source to be rejected")
	}
}

func TestCreateClean(t *testing.T) {
	if runtime.GOOS == testWindows {
		t.Skip("fake gh executable uses a POSIX shell")
	}
	tests := []struct {
		name       string
		script     string
		wantErr    bool
		wantSource bool
	}{
		{name: "successful creation", script: "exit 0"},
		{name: "failed creation", script: "exit 1", wantErr: true, wantSource: true},
		{name: "source edited during creation", script: `printf edited > "$CLEAN_TEST_SOURCE"`, wantErr: true, wantSource: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bin := t.TempDir()
			// #nosec G306 -- isolated fake gh executable must be executable
			if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\n"+test.script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			path := filepath.Join(t.TempDir(), sourceTestFile)
			if err := os.WriteFile(path, []byte("---\ntitle: Issue\n---\n"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CLEAN_TEST_SOURCE", path)
			err := Create(CreateOptions{File: path, Clean: true})
			if (err != nil) != test.wantErr {
				t.Errorf("create error = %v, wantErr %v", err, test.wantErr)
			}
			if test.name == "source edited during creation" && err != nil && !strings.Contains(err.Error(), "issue created, but source cleanup failed") {
				t.Errorf("cleanup error does not report successful creation: %v", err)
			}
			_, statErr := os.Stat(path)
			if (statErr == nil) != test.wantSource {
				t.Errorf("source existence error = %v, want retained %v", statErr, test.wantSource)
			}
		})
	}
}

func TestCreateCleanRejectsRemoteSources(t *testing.T) {
	for _, options := range []CreateOptions{{File: sourceTestFile, Branch: "stable", Clean: true}, {File: sourceTestFile, Gist: "gist", Clean: true}, {File: sourceTestFile, Repo: "owner/example", Clean: true}} {
		if err := Create(options); err == nil || !strings.Contains(err.Error(), "--clean requires a local file") {
			t.Errorf("Create(%+v) error = %v", options, err)
		}
	}
}

func TestCreateIssueCommaMetadata(t *testing.T) {
	if runtime.GOOS == testWindows {
		t.Skip("fake gh executable uses a POSIX shell")
	}
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "gh.log")
	// #nosec G306 -- isolated fake gh executable must be executable
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CREATE_TEST_LOG\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("CREATE_TEST_LOG", logPath)
	metadata := &IssueMetadata{Title: "Issue", Labels: []Label{{Name: "help, needed"}}, Projects: []string{"Board, one"}}
	if err := createIssue(metadata, ""); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- path is inside the test's temporary directory
	log, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(log), "--body\n\n") || !strings.Contains(string(log), "--label\n\"help, needed\"\n") || !strings.Contains(string(log), "--project\n\"Board, one\"\n") {
		t.Errorf("unexpected create arguments: %q, error = %v", log, err)
	}
}

func TestCreateMetadataPreflight(t *testing.T) {
	if runtime.GOOS == testWindows {
		t.Skip("fake gh executable uses a POSIX shell")
	}
	tests := []struct {
		name         string
		frontmatter  string
		labelPages   string
		createLabels bool
		wantErr      bool
		wantLabel    bool
		wantIssue    bool
	}{
		{name: "missing label without opt-in", frontmatter: "labels:\n  - name: missing\n", labelPages: emptyTestPages, wantErr: true},
		{name: "name-only label with opt-in", frontmatter: "labels:\n  - name: missing\n", labelPages: emptyTestPages, createLabels: true, wantLabel: true, wantIssue: true},
		{name: "existing label on later page", frontmatter: "labels:\n  - name: EXISTING\n", labelPages: `[[{"name":"other"}],[{"name":"existing"}]]`, wantIssue: true},
		{name: "invalid milestone before label creation", frontmatter: "milestone: missing\nlabels:\n  - name: missing\n", labelPages: emptyTestPages, createLabels: true, wantErr: true},
		{name: "unknown field", frontmatter: "label: typo\n", labelPages: emptyTestPages, wantErr: true},
		{name: "invalid yaml", frontmatter: "labels: [\n", labelPages: emptyTestPages, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bin := t.TempDir()
			logPath := filepath.Join(t.TempDir(), "gh.log")
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$CREATE_TEST_LOG"
case "$1/$2" in
  api/--paginate) case "$4" in *milestones*) printf '%s' '[[]]' ;; *labels*) printf '%s' "$CREATE_TEST_LABELS" ;; *) exit 1 ;; esac ;;
  api/--method) printf '%s' '{}' ;;
  issue/create) exit 0 ;;
  *) exit 1 ;;
esac
`
			// #nosec G306 -- isolated fake gh executable must be executable
			if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			t.Setenv("CREATE_TEST_LOG", logPath)
			t.Setenv("CREATE_TEST_LABELS", test.labelPages)
			path := filepath.Join(t.TempDir(), sourceTestFile)
			if err := os.WriteFile(path, []byte("---\ntitle: Issue\n"+test.frontmatter+"---\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err := Create(CreateOptions{File: path, CreateLabels: test.createLabels})
			if (err != nil) != test.wantErr {
				t.Errorf("create error = %v, wantErr %v", err, test.wantErr)
			}
			// #nosec G304 -- path is inside the test's temporary directory
			log, readErr := os.ReadFile(logPath)
			if readErr != nil && (!test.wantErr || !os.IsNotExist(readErr)) {
				t.Fatal(readErr)
			}
			if strings.Contains(string(log), "api --method POST") != test.wantLabel || strings.Contains(string(log), "issue create") != test.wantIssue {
				t.Errorf("unexpected fake GitHub commands: %q", log)
			}
		})
	}
}

func cleanupTestRunner(path string, failUpdate, editSource bool) githubRunner {
	return func(args []string, _ string) ([]byte, error) {
		if args[0] == pushAPICommand && args[1] == pushGraphQLCommand {
			return []byte(fakeProjects), nil
		}
		if args[0] == pushAPICommand {
			return []byte(fakeIssue), nil
		}
		if failUpdate {
			return nil, fmt.Errorf("update failed")
		}
		if editSource {
			if err := os.WriteFile(path, []byte("edited"), 0600); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
}

func TestPushClean(t *testing.T) {
	tests := []struct {
		name       string
		failUpdate bool
		editSource bool
		wantErr    bool
		wantFile   bool
	}{
		{name: "successful update"},
		{name: "failed update", failUpdate: true, wantErr: true, wantFile: true},
		{name: "source edited during update", editSource: true, wantErr: true, wantFile: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "7-draft.md")
			if err := os.WriteFile(path, []byte("---\ntitle: New\n---\n"+oldBody), 0600); err != nil {
				t.Fatal(err)
			}
			drafts, err := preparePushDrafts(directory, []int{7})
			if err != nil {
				t.Fatal(err)
			}
			run := cleanupTestRunner(path, test.failUpdate, test.editSource)
			err = pushDraftsWithOptions(drafts, PushOptions{Clean: true}, io.Discard, run)
			if (err != nil) != test.wantErr {
				t.Errorf("push error = %v, wantErr %v", err, test.wantErr)
			}
			if test.editSource && err != nil && !strings.Contains(err.Error(), "updated, but source cleanup failed") {
				t.Errorf("cleanup error does not report successful update: %v", err)
			}
			_, statErr := os.Stat(path)
			if (statErr == nil) != test.wantFile {
				t.Errorf("source existence error = %v, want retained %v", statErr, test.wantFile)
			}
		})
	}
}

func TestPushCleanBatch(t *testing.T) {
	directory := t.TempDir()
	for _, number := range []int{7, 8} {
		path := filepath.Join(directory, fmt.Sprintf("%d-draft.md", number))
		if err := os.WriteFile(path, []byte("---\ntitle: New\n---\n"+oldBody), 0600); err != nil {
			t.Fatal(err)
		}
	}
	drafts, err := preparePushDrafts(directory, []int{7, 8})
	if err != nil {
		t.Fatal(err)
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
		if args[2] == "7" {
			return nil, fmt.Errorf("update failed")
		}
		return nil, nil
	}
	err = pushDraftsWithOptions(drafts, PushOptions{Clean: true}, io.Discard, run)
	if err == nil || !strings.Contains(err.Error(), "issue #7") {
		t.Fatalf("expected issue 7 error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "7-draft.md")); err != nil {
		t.Errorf("failed issue source should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "8-draft.md")); !os.IsNotExist(err) {
		t.Errorf("successful issue source should be removed: %v", err)
	}
}

func TestPushCleanUnchangedIssue(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "7-draft.md")
	content := "---\ntitle: Old\nassign: [alice, Copilot]\nlabels:\n  - name: defect\n  - name: old\nmilestone: Old milestone\nprojects: [Old project]\n---\n" + oldBody
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	drafts, err := preparePushDrafts(directory, []int{7})
	if err != nil {
		t.Fatal(err)
	}
	run := func(args []string, _ string) ([]byte, error) {
		if args[0] == pushAPICommand && args[1] == pushGraphQLCommand {
			return []byte(fakeProjects), nil
		}
		if args[0] == pushAPICommand {
			return []byte(fakeIssue), nil
		}
		return nil, fmt.Errorf("unexpected write: %v", args)
	}
	if err := pushDraftsWithOptions(drafts, PushOptions{Clean: true}, io.Discard, run); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("unchanged issue's source should be removed: %v", err)
	}
}

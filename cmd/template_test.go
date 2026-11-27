package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lakruzz/gh-mkissue/templates"
)

const outputFlag = "--out"

func TestTemplateOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "minimal", args: []string{outputFlag, "-"}, want: templates.MinimalTemplate},
		{name: "elaborate", args: []string{"--elaborate", outputFlag, "-"}, want: templates.ElaborateTemplate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newTemplateCmd()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if output.String() != test.want {
				t.Errorf("output = %q, want %q", output.String(), test.want)
			}
		})
	}
}

func TestTemplateDefaultFile(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	command := newTemplateCmd()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile("template.issue.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != templates.MinimalTemplate || output.String() != "Created template.issue.md\n" {
		t.Errorf("content = %q, output = %q", content, output.String())
	}
	command = newTemplateCmd()
	command.SetArgs([]string{})
	if err := command.Execute(); err == nil {
		t.Fatal("expected an error when the default file already exists")
	}
}

func TestTemplateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issue.md")
	command := newTemplateCmd()
	command.SetArgs([]string{"--elaborate", outputFlag, path})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- path is inside the test's temporary directory
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != templates.ElaborateTemplate {
		t.Error("file does not contain the elaborate template")
	}
	command = newTemplateCmd()
	command.SetArgs([]string{outputFlag, path})
	if err := command.Execute(); err == nil {
		t.Fatal("expected an error when the output file already exists")
	}
	// #nosec G304 -- path is inside the test's temporary directory
	content, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != templates.ElaborateTemplate {
		t.Error("existing file was modified")
	}
}

func TestTemplateCommandRouting(t *testing.T) {
	const templateCommand = "template"
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{name: "template without file", args: []string{templateCommand, outputFlag, "-"}, want: templates.MinimalTemplate},
		{name: "create requires file", args: []string{"create"}, wantErr: `required flag(s) "file" not set`},
		{name: "old form rejected", args: []string{"mkissue", "--file", "legacy.md"}, wantErr: `unknown command "mkissue"`},
		{name: "template rejects file flag", args: []string{templateCommand, "--file", "issue.md"}, wantErr: "unknown flag: --file"},
		{name: "template rejects arguments", args: []string{templateCommand, "issue.md"}, wantErr: "unknown command"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			rootCmd.SetOut(&output)
			rootCmd.SetErr(&output)
			rootCmd.SetArgs(test.args)
			defer rootCmd.SetOut(nil)
			defer rootCmd.SetErr(nil)
			defer rootCmd.SetArgs(nil)
			err := rootCmd.Execute()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if output.String() != test.want {
				t.Errorf("output = %q, want %q", output.String(), test.want)
			}
		})
	}
}

func TestRootCommands(t *testing.T) {
	for _, name := range []string{"create", "template", "pull", "push"} {
		command, _, err := rootCmd.Find([]string{name})
		if err != nil || command == rootCmd || command.Name() != name {
			t.Errorf("root command %q = %v, error = %v", name, command, err)
		}
	}
	if command, _, err := rootCmd.Find([]string{"mkissue"}); err == nil && command != rootCmd {
		t.Error("removed nested mkissue command is still registered")
	}
}

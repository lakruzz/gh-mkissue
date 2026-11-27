package cmd

import (
	"io"
	"testing"

	"github.com/lakruzz/gh-mkissue/cmd/mkissue"
)

func TestCreateCommand(t *testing.T) {
	const branchName = "main"
	const issuePath = "issue.md"
	const repository = "owner/repo"
	tests := []struct {
		name    string
		args    []string
		want    mkissue.CreateOptions
		wantErr bool
	}{
		{name: "local options", args: []string{"--file=" + issuePath, "--clean", "--create-labels"}, want: mkissue.CreateOptions{File: issuePath, Clean: true, CreateLabels: true}},
		{name: "remote source", args: []string{"-f", issuePath, "-r", repository, "-b", branchName}, want: mkissue.CreateOptions{File: issuePath, Repo: repository, Branch: branchName}},
		{name: "missing file", wantErr: true},
		{name: "gist and branch", args: []string{"-f", issuePath, "-g", "gist", "-b", branchName}, wantErr: true},
		{name: "gist and repo", args: []string{"-f", issuePath, "-g", "gist", "-r", repository}, wantErr: true},
		{name: "positional rejected", args: []string{issuePath}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got mkissue.CreateOptions
			called := false
			command := newCreateCmdWith(func(options mkissue.CreateOptions) error {
				called = true
				got = options
				return nil
			})
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SilenceUsage = true
			command.SilenceErrors = true
			command.SetArgs(test.args)
			err := command.Execute()
			if (err != nil) != test.wantErr || called == test.wantErr || (!test.wantErr && got != test.want) {
				t.Errorf("create options = %+v, called = %v, error = %v; want %+v, error %v", got, called, err, test.want, test.wantErr)
			}
		})
	}
}

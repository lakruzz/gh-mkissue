package cmd

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/lakruzz/gh-mkissue/cmd/mkissue"
)

func TestPushCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []int
		wantDir string
		force   bool
		clean   bool
		labels  bool
		wantErr bool
	}{
		{name: "single", args: []string{"7"}, want: []int{7}},
		{name: "list and flags", args: []string{"4,5,4", "--dir=workspace-drafts", "--force"}, want: []int{4, 5}, wantDir: "workspace-drafts", force: true},
		{name: "cleanup and labels", args: []string{"7", "--clean", "--create-labels"}, want: []int{7}, clean: true, labels: true},
		{name: "open rejected", args: []string{"open"}, wantErr: true},
		{name: "spaces rejected", args: []string{"1, 2"}, wantErr: true},
		{name: "empty directory", args: []string{"7", "--dir="}, wantErr: true},
		{name: "no arguments", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called []int
			command := newPushCmdWith(func(numbers []int, options mkissue.PushOptions, _ io.Writer) error {
				called = numbers
				if options.Directory != test.wantDir || options.Force != test.force || options.Clean != test.clean || options.CreateLabels != test.labels {
					t.Errorf("options = %+v; want directory %q, force %v, clean %v, labels %v", options, test.wantDir, test.force, test.clean, test.labels)
				}
				return nil
			})
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SilenceUsage = true
			command.SilenceErrors = true
			command.SetArgs(test.args)
			err := command.Execute()
			if (err != nil) != test.wantErr || !reflect.DeepEqual(called, test.want) {
				t.Errorf("called = %v, error = %v; want %v, error %v", called, err, test.want, test.wantErr)
			}
		})
	}
}

func TestPushRootRouting(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "open"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	defer rootCmd.SetArgs(nil)
	defer rootCmd.SetOut(nil)
	defer rootCmd.SetErr(nil)
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "comma-separated positive integers") {
		t.Fatalf("expected push selection validation, got %v", err)
	}
}

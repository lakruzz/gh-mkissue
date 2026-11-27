package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseIssueNumbers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{name: "single", input: "7", want: []int{7}},
		{name: "list and duplicates", input: "4,5,89,4", want: []int{4, 5, 89}},
		{name: "leading zeros", input: "07,7", want: []int{7}},
		{name: "empty", input: "", wantErr: true},
		{name: "space", input: "4, 5", wantErr: true},
		{name: "empty item", input: "4,,5", wantErr: true},
		{name: "trailing comma", input: "4,", wantErr: true},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "plus", input: "+1", wantErr: true},
		{name: "overflow", input: "999999999999999999999999", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseIssueNumbers(test.input)
			if (err != nil) != test.wantErr || !reflect.DeepEqual(got, test.want) {
				t.Errorf("parseIssueNumbers(%q) = %v, %v; want %v, error %v", test.input, got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestPullIssuesContinuesAfterErrors(t *testing.T) {
	var called []int
	var output bytes.Buffer
	pull := func(number int, directory string, force bool) (string, error) {
		called = append(called, number)
		if directory != "drafts" || !force {
			t.Errorf("unexpected options: %q, %v", directory, force)
		}
		if number == 5 {
			return "", errors.New("not found")
		}
		return fmt.Sprintf("%d.md", number), nil
	}
	err := pullIssues(&output, []int{4, 5, 89}, "drafts", true, pull)
	if !reflect.DeepEqual(called, []int{4, 5, 89}) || err == nil || !strings.Contains(err.Error(), "issue #5: not found") || !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("called = %v, error = %v", called, err)
	}
	if output.String() != "Created 4.md\nCreated 89.md\n" {
		t.Errorf("output = %q", output.String())
	}
}

func TestPullCommandSelection(t *testing.T) {
	const openSelection = "open"
	tests := []struct {
		name       string
		args       []string
		open       []int
		want       []int
		wantOutput string
		wantErr    bool
		wantLists  int
	}{
		{name: "csv", args: []string{"4,5,4"}, want: []int{4, 5}},
		{name: "invalid csv", args: []string{"4, 5"}, wantErr: true},
		{name: openSelection, args: []string{openSelection}, open: []int{4, 5}, want: []int{4, 5}, wantLists: 1},
		{name: "no open issues", args: []string{openSelection}, wantOutput: "No open issues found.\n", wantLists: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called []int
			listCalls := 0
			var output bytes.Buffer
			command := newPullCmdWith(func() ([]int, error) {
				listCalls++
				return test.open, nil
			}, func(number int, _ string, _ bool) (string, error) {
				called = append(called, number)
				return fmt.Sprintf("%d.md", number), nil
			})
			command.SetOut(&output)
			command.SetErr(io.Discard)
			command.SilenceUsage = true
			command.SilenceErrors = true
			command.SetArgs(test.args)
			err := command.Execute()
			if (err != nil) != test.wantErr || !reflect.DeepEqual(called, test.want) || listCalls != test.wantLists {
				t.Errorf("called = %v, list calls = %d, error = %v", called, listCalls, err)
			}
			if test.wantOutput != "" && output.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", output.String(), test.wantOutput)
			}
		})
	}
}

func TestPullInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing"},
		{name: "zero", args: []string{"0"}},
		{name: "not a number", args: []string{"abc"}},
		{name: "too many", args: []string{"1", "2"}},
		{name: "empty directory", args: []string{"1", "--dir", ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newPullCmd()
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SetArgs(test.args)
			if err := command.Execute(); err == nil {
				t.Fatal("expected an invalid argument error")
			}
		})
	}
}

func TestPullFlags(t *testing.T) {
	command := newPullCmd()
	for _, name := range []string{"dir", "force"} {
		if command.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
}

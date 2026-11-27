<!-- cspell:ignore gofmt golangci  -->

# gh-mkissue

A GitHub CLI extension for creating and managing issues from Markdown files.

**See the full documentation live at [dev.lakruzz.com/gh-mkissue](https://dev.lakruzz.com/gh-mkissue/)**

## Installation

Install as a GitHub CLI extension:

```bash
gh extension install lakruzz/gh-mkissue
```

## Usage

### `mkissue` - Create GitHub Issue from Markdown File

Create a GitHub issue from a markdown file with YAML frontmatter:

```bash
gh mkissue create --file path/to/issue.md
# or using short form
gh mkissue create -f path/to/issue.md
```

#### Creating a Draft

Create a minimal issue draft in `template.issue.md` in the current directory:

```bash
gh mkissue template
gh mkissue template --out issue.md
gh mkissue template --out -
```

Use `--out -` to print the draft to standard output. Add `--elaborate` to include the full template reference. Output files must not already exist. Edit the draft to provide a title and issue body before creating the issue. See the [template guide](docs/content/commands/template.md) for details.

#### Pulling an Existing Issue

```bash
gh mkissue pull 7
gh mkissue pull 4,5,89
gh mkissue pull open
```

This saves issues and their supported metadata under the repository's `issues/` directory. Use `--dir` to choose another directory and `--force` to replace an existing draft. See the [pull guide](docs/content/commands/pull.md) for details.

#### Pushing an Edited Issue

```bash
gh mkissue push 7
gh mkissue push 4,5,89 --dir path/to/drafts
```

Push synchronizes the issue from its local draft. Body changes on issues with comments require `--force`. Use `--clean` to remove unchanged drafts after successful updates and `--create-labels` to create missing labels. See the [push guide](docs/content/commands/push.md) for full-state and safety details.

#### Reading from a Git Branch

You can read the issue file from a specific git branch without checking it out to your filesystem:

```bash
gh mkissue create --file path/to/issue.md --branch secret
# or using short form
gh mkissue create -f path/to/issue.md -b secret
```

This is useful for keeping issue templates in a separate orphan branch.

#### Reading from a GitHub Repository

You can read the issue file from a specific GitHub repository:

```bash
gh mkissue create --file path/to/issue.md --repo owner/repo
# or using short form
gh mkissue create -f path/to/issue.md -r owner/repo
```

Optionally combine with `--branch` to read from a specific branch in that repository:

```bash
gh mkissue create --file path/to/issue.md --repo owner/repo --branch my-branch
```

If `--branch` is not specified, the repository's default branch is used.

#### Flag Rules

```text
gh mkissue create --file <file> [--gist <gist-id>]
```

- `--file` is required when creating an issue
- `--branch` is optional
- `--gist` and `--repo` are mutually exclusive
- `--branch` is not valid with `--gist`
- `--clean` is only valid for a local file source
- `--create-labels` allows creation of missing labels

#### Reading from a GitHub Gist

You can read the issue file from a GitHub Gist:

```bash
gh mkissue create --file filename.md --gist <gist-id>
# or using short form
gh mkissue create -f filename.md -g <gist-id>
```

The `--gist` flag accepts a 32-character hexadecimal gist ID.

#### Issue File Format

The issue file must follow the format specified in [`templates/template.issue.md`](templates/template.issue.md). This template defines the contract for issue files:

- **Frontmatter** (YAML): Contains metadata like title, assignees, labels, milestone, and projects
- **Body** (Markdown): The issue description/content

**Example:**

```yaml
---
title: "My Issue Title"
assign:
  - "@me"
labels:
  - name: "bug"
  - name: "priority-high"
    color: "d73a4a"
    desc: "High priority issues"
milestone: "v1.0"
projects:
  - "Main Project"
---
## Issue Description

This is the issue body written in Markdown.

- Supports lists
- **Bold text**
- And all other Markdown features
```

See the [create guide](docs/content/commands/create.md) and [`templates/template.issue.md`](templates/template.issue.md) for the complete specification and all supported fields.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for developer documentation and guidelines.

## License

This project is intended for use as specified in the repository.

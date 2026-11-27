---
title: Create an issue
description: Create a GitHub issue from a Markdown file.
sidebar:
  order: 3
  label: Create
---

`gh mkissue create` creates a GitHub issue from a Markdown file containing YAML frontmatter and an optional Markdown body. It uses the authenticated GitHub CLI and creates the issue in the repository where the command is run.

```bash
gh mkissue create --file template.issue.md
gh mkissue create --file template.issue.md --clean
```

The `--file` flag is required and also has the short form `-f`. Use `--clean` (`-c`) to delete an unchanged local source after successful creation; it cannot be used with a branch, repository, or gist source. If the file was changed during creation, it is retained and cleanup reports an error even though the issue was created. The file needs a nonempty `title` in its frontmatter. For example:

```markdown
---
title: "Improve the issue workflow"
assign: ["@me"]
labels:
  - name: "enhancement"
milestone: "Next release"
projects: ["Roadmap"]
---

Describe the issue here.
```

All fields except `title` are optional. Assignees use GitHub login names or `@me`. Labels must already exist unless `--create-labels` is supplied; optional `color` and `desc` apply when creating a missing label. Named milestones and projects must already exist. Labels created before a later failure are not rolled back. See the [full template reference](https://github.com/lakruzz/gh-mkissue/blob/main/templates/template.issue.md) for the supported fields.

## Read from another source

Read a file from a branch of the current repository:

```bash
gh mkissue create --file path/to/issue.md --branch feature
```

Read from another repository, optionally selecting a branch. Without `--branch`, its default branch is used:

```bash
gh mkissue create --file path/to/issue.md --repo owner/repo
gh mkissue create --file path/to/issue.md --repo owner/repo --branch feature
```

Read a file from a gist using its 32-character hexadecimal ID:

```bash
gh mkissue create --file issue.md --gist GIST_ID
```

The source flags also have short forms: `-b` for `--branch`, `-r` for `--repo`, and `-g` for `--gist`. `--gist` cannot be combined with `--repo` or `--branch`. These flags select where to read the file; they do not change the repository where the issue is created.

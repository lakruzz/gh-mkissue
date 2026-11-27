---
title: mkissue for GitHub CLI
description: Create and manage GitHub issues from Markdown files.
sidebar:
  order: 1
redirectFrom: /
# cspell:ignore lsewhere ublish yndicate
---

## Purpose

`gh-mkissue` is a GitHub CLI extension for creating, pulling, and updating issues from Markdown files.

## Install

Install the extension after installing and authenticating the GitHub CLI:

```bash
gh auth login
gh extension install lakruzz/gh-mkissue
```

Run the commands from the Git repository where you want to create or update issues. Your GitHub account needs permission to access that repository and its issue metadata.

## Create an issue

Generate a starter file in the current directory:

```bash
gh mkissue template
```

Use `--out` to choose another file or `--out -` to print the draft. Add `--elaborate` for the full template reference. Edit `template.issue.md` to provide a title and issue description. The file has YAML frontmatter followed by a Markdown body:

```markdown
---
title: "Improve the issue workflow"
assign: ["@me"]
labels:
  - name: "enhancement"
---

Describe the issue here.
```

Create the issue from that file:

```bash
gh mkissue create --file template.issue.md
```

The title is required. Assignees, labels, milestone, and projects are optional. Labels, milestones, and projects must already exist unless you explicitly use `--create-labels` for missing labels. The `--clean` flag removes an unchanged local source after successful creation.

You can read the source from a branch, another repository, or a gist using `--branch`, `--repo`, or `--gist`. These options select the source file, not the destination repository, and cannot be combined with `--clean`.

## Edit an existing issue

Pull an issue into the repository's `issues/` directory, edit the resulting Markdown file, and push it back:

```bash
gh mkissue pull 7
gh mkissue push 7
```

Pull accepts comma-separated issue numbers without spaces, or `open` for all open issues. Use `--dir` to select another directory, and `--force` to replace an existing local draft. The default `issues/` directory receives a `.gitignore` if needed; existing ignore files and already tracked files are left unchanged.

Push treats the draft as the desired complete state; omitted optional fields remove the corresponding issue metadata. It accepts comma-separated issue numbers, and `--dir` selects the source directory. Changing the description of an issue with comments requires `--force`. Use `--clean` to remove unchanged drafts after successful updates.

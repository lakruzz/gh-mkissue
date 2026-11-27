---
title: Pull an issue
description: Save existing GitHub issues as local Markdown drafts.
sidebar:
  order: 1
  label: Pull
---

```bash
gh mkissue pull 7
gh mkissue pull 4,5,89
gh mkissue pull open
gh mkissue pull 7 --dir path/to/drafts
gh mkissue pull 7 --force
```

Supply a single issue number, a comma-separated list without spaces, or `open` for all currently open issues. Duplicate numbers are pulled once. The command retrieves each issue from the current GitHub repository and saves it under `issues/` at the Git repository root, using a filename such as `7-example_issue.md`. The file contains the title, body, assignees, existing label names, milestone, and project titles. Comments, issue state, and project status are not included.

Batch pulls continue after individual errors and return a nonzero exit code if any issue fails. Successfully written drafts remain in place. If there are no open issues, no files are created.

Use `--dir` (`-d`) to choose another output directory; relative paths are resolved from the current directory. New directories receive a `.gitignore` that ignores their contents while allowing the ignore file itself to be tracked. Existing custom directories are left unchanged, so check their ignore rules. For the default `issues/` directory, a missing `.gitignore` is created. Existing ignore files are never modified, and Git continues to track any files already tracked.

An existing draft for the same issue number is not overwritten unless `--force` (`-f`) is supplied. When the title has changed, `--force` updates the existing filename. If multiple drafts match the issue number, resolve them before using `--force`.

Pulling only creates a local file; it does not change the GitHub issue.

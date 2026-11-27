---
title: Push an issue
description: Update existing GitHub issues from local Markdown drafts.
sidebar:
  order: 2
  label: Push
---

```bash
gh mkissue push 7
gh mkissue push 4,5,89 --dir path/to/drafts
gh mkissue push 7 --force
gh mkissue push 7 --clean --create-labels
```

Push accepts one issue number or a comma-separated list without spaces; `open` is not supported. It reads drafts from `issues/` at the repository root, or from `--dir` (`-d`), which resolves relative paths from the current directory. Each selected issue must have exactly one matching `<number>-*.md` file. The entire batch is checked for missing, ambiguous, or invalid drafts before any updates.

The draft is the desired complete state: push synchronizes the title, body, assignees, labels, milestone, and projects. Omitted optional fields remove the corresponding remote metadata. Issue state, comments, and project status are not changed. Missing labels fail unless `--create-labels` is supplied. Project access requires the appropriate GitHub CLI authorization.

If the body has changed and the issue has comments, the update is rejected unless `--force` (`-f`) is supplied. Metadata-only updates are allowed when the body is unchanged. Force does not bypass draft validation or ambiguity checks. A comment can still be added between the check and the update.

After local preflight, batch pushes continue after individual errors and return a nonzero exit code if any issue fails. Successful updates remain in place. GitHub operations are not transactional: labels created before a later failure may remain, and an issue update may be partially applied if GitHub reports an error.

Use `--clean` (`-c`) to delete each unchanged local draft after its issue is successfully updated. Failed updates keep their drafts. If a draft was edited or replaced during the update, it is retained and cleanup reports an error; the issue may already have been updated.

---
title: Create issues with mkissue
description: Generate an issue draft and create a GitHub issue from Markdown.
---

`gh utils mkissue` creates a GitHub issue from a Markdown file containing YAML frontmatter and an optional Markdown body. It uses the authenticated GitHub CLI and creates the issue in the repository where the command is run.

## Generate a draft

```bash
gh utils mkissue template
```

This creates `template.issue.md` in the current directory. The draft contains the supported frontmatter fields; optional fields are commented out. Fill in the required title and add the issue body after the closing `---`.

Choose a different destination, include the full template reference, or print to standard output:

```bash
gh utils mkissue template --out issue.md
gh utils mkissue template --elaborate --out issue.md
gh utils mkissue template --out -
```

The short forms are `-o` for `--out` and `-e` for `--elaborate`. Existing files are never overwritten. The elaborate version includes explanatory text in the Markdown body; replace it with your issue description before creating the issue.

Earlier versions printed the template by default. Use `--out -` to retain that behavior in scripts.

## Pull an existing issue

```bash
gh utils mkissue pull 7
gh utils mkissue pull 4,5,89
gh utils mkissue pull open
gh utils mkissue pull 7 --dir path/to/drafts
gh utils mkissue pull 7 --force
```

Supply a single issue number, a comma-separated list without spaces, or `open` for all currently open issues. Duplicate numbers are pulled once. The command retrieves each issue from the current GitHub repository and saves it under `issues/` at the Git repository root, using a filename such as `7-example_issue.md`. The file contains the title, body, assignees, existing label names, milestone, and project titles. Comments, issue state, and project status are not included.

Batch pulls continue after individual errors and return a nonzero exit code if any issue fails. Successfully written drafts remain in place. If there are no open issues, no files are created.

Use `--dir` (`-d`) to choose another output directory; relative paths are resolved from the current directory. New directories receive a `.gitignore` that ignores their contents while allowing the ignore file itself to be tracked. Existing custom directories are left unchanged, so check their ignore rules. For the default `issues/` directory, a missing `.gitignore` is created. Existing ignore files are never modified, and Git continues to track any files already tracked.

An existing draft for the same issue number is not overwritten unless `--force` (`-f`) is supplied. When the title has changed, `--force` updates the existing filename. If multiple drafts match the issue number, resolve them before using `--force`.

Pulling only creates a local file; it does not change the GitHub issue. Updating an existing issue from an edited draft is not yet supported.

## Create an issue

```bash
gh utils mkissue --file template.issue.md
```

The `--file` flag is required and also has the short form `-f`. The file needs a nonempty `title` in its frontmatter. For example:

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

All fields except `title` are optional. Assignees use GitHub login names or `@me`. Labels with only a `name` must already exist; adding `color` or `desc` allows a missing label to be created. Named milestones and projects must already exist. See the [full template reference](../../../templates/template.issue.md) for the supported fields.

## Read from another source

Read a file from a branch of the current repository:

```bash
gh utils mkissue --file path/to/issue.md --branch feature
```

Read from another repository, optionally selecting a branch. Without `--branch`, its default branch is used:

```bash
gh utils mkissue --file path/to/issue.md --repo owner/repo
gh utils mkissue --file path/to/issue.md --repo owner/repo --branch feature
```

Read a file from a gist using its 32-character hexadecimal ID:

```bash
gh utils mkissue --file issue.md --gist GIST_ID
```

The source flags also have short forms: `-b` for `--branch`, `-r` for `--repo`, and `-g` for `--gist`. `--gist` cannot be combined with `--repo` or `--branch`. These flags select where to read the file; they do not change the repository where the issue is created.

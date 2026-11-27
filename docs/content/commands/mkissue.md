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

---
title: Generate a template
description: Generate a minimal issue draft or the full template reference.
sidebar:
  order: 4
  label: Template
---

```bash
gh mkissue template
```

This creates `template.issue.md` in the current directory. The draft contains the supported frontmatter fields; optional fields are commented out. Fill in the required title and add the issue body after the closing `---`.

Choose a different destination, include the full template reference, or print to standard output:

```bash
gh mkissue template --out issue.md
gh mkissue template --elaborate --out issue.md
gh mkissue template --out -
```

The short forms are `-o` for `--out` and `-e` for `--elaborate`. Existing files are never overwritten. The elaborate version includes explanatory text in the Markdown body; replace it with your issue description before creating the issue.

Earlier versions printed the template by default. Use `--out -` to retain that behavior in scripts.

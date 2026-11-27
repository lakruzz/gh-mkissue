---
name: mkissue
description: Draft and review a GitHub issue specification with gh utils mkissue. Use when asked to spec, write, or create a GitHub issue from a problem or idea.
---

# Specify a GitHub issue

## Prepare

1. Check that the GitHub CLI is available and that the `utils` extension is installed with `gh extension list`. If the extension is missing, install it with `gh ext install lakruzz/gh-utils`. Report installation or authentication errors instead of assuming success.
2. Read the mkissue guide. When working in the `lakruzz/gh-utils` checkout, read `docs/content/commands/mkissue.md` there. Otherwise fetch it with:

   ```sh
   gh api -H 'Accept: application/vnd.github.raw' repos/lakruzz/gh-utils/contents/docs/content/commands/mkissue.md
   ```

   The remote guide is from the repository's default branch and may differ from the installed extension version. If it is unavailable, tell the user and consult `gh utils mkissue --help` and `gh utils mkissue template --help` before proceeding.

3. Inspect the target repository's issue conventions and any relevant code or documentation. Ask focused questions if the problem, intended outcome, or target repository is unclear.

## Draft and review

1. Obtain the minimal frontmatter with `gh utils mkissue template --out -`. Follow the guide for supported fields. Use a meaningful title and include optional metadata only when known or requested.
2. Write an issue specification that captures the problem, desired outcome, relevant context, and verifiable acceptance criteria. Match the repository's conventions and avoid inventing requirements.
3. Save the draft to the path requested by the user. If none was specified, choose a descriptive `.issue.md` path consistent with the repository; never overwrite an existing file without permission.
4. Present the draft and its path for review. Revise it based on feedback.

## Create the issue

Only create a GitHub issue after the user explicitly approves publication. Confirm the intended destination repository with the user, check GitHub CLI authentication, and run `gh utils mkissue create --file <draft-path>` from that repository. Source flags select where to read the draft, not where the issue is created. Report the resulting issue URL or any error.

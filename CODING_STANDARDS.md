# Coding standards (for reviewers)

Judgement calls the linters and CI cannot make. Apply every rule to the diff under review; `make check` covers the rest.

- **Behaviour matches `main`.** A review fix keeps user-visible behaviour (flags, stdin handling, exit codes, output format) identical to `main` unless the issue being implemented asks for the change. When a behaviour question comes up, raise it with the maintainer and leave the behaviour as `main` has it.
- **Docs ship with the change.** A PR that adds or changes a command or flag updates `README.md` and `skills/peppol/` (`SKILL.md` + the matching `references/*.md`) in the same PR.
- **`--json` stdout is one JSON document.** With `--json`, stdout carries a single parseable JSON document; prompts, progress and warnings go to stderr.

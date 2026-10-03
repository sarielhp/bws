# Archived scripts

These one-off Ruby scripts and patches were used during earlier refactors and
error-handling edits. They are retained for reference and are not part of the build or test
workflow.

- `split_*.rb`: split Go source and test files into smaller files.
- `fix_errors*.rb`: rewrite ignored error assignments in source and tests.
- `fix_proxy*.patch`: historical proxy error-handling edits.

The scripts assume older source layouts and paths relative to the repository
root. Running them against the current tree can overwrite files or fail.

Maintained developer commands live in `tools/`.

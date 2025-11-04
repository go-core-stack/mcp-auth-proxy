# AI Agent Context

This repository follows the conventions below so that automated agents and human contributors write and review changes consistently.

## Coding Style & Naming Conventions
- Target Go 1.24 and rely on `go fmt` to normalize formatting; fixes must pass `golangci-lint run` before merging.
- Stick to the default Go toolchain layout: tabs for indentation, UTF-8 ASCII source files, and one public type or function per file when practical.
- Keep package and directory names lower_snake (`pkg/controller`, `pkg/auth`); exported identifiers use PascalCase and begin with a leading doc comment.
- Mirror configuration struct fields to their YAML keys using lowerCamel case, and keep generated protobuf clients under `api/`—regenerate them with `buf`/`protoc` instead of editing manually.
- Place table-driven tests in `_test.go` files alongside the code; name suites `Test<Feature><Scenario>` and use `t.Run` for edge cases so `go test ./...` stays readable for agents.

## Commit & Pull Request Guidelines
- Configure git author info and sign every commit with `git commit -s`; the signed-off trailer is required for compliance.
- Write commit subjects ≤55 characters in the imperative mood (for example, `Add tenant cache`) and separate the body with a blank line.
- Wrap commit bodies at 72 characters, explain the what and why, and finish with any trackers (`Jira: CLOUD-123`, `Refs: #45`) followed by the `Signed-off-by` line added by `-s`.
- Develop on purpose-specific branches such as `feature/oauth-proxy` or `fix/token-refresh`; update in-review work with `git commit --amend` or an interactive rebase and `git push --force-with-lease`.
- Keep pull request descriptions concise: lead with behavioral changes, link issues or design docs, attach evidence for new endpoints, and confirm `go test ./...` (and `make build` when available) before requesting review.
- After merge, append the tracking issue ID (for example, `(#123)`) to the PR title to keep release notes traceable.

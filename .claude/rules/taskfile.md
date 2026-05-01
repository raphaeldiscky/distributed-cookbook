# Rule: Keep `taskfile.yml` thin — long shell goes in `scripts/`

`taskfile.yml` is a registry of *what* tasks exist, not *how* they
work. Long shell embedded in YAML loses syntax highlighting, can't be
shellchecked, can't be tested standalone, and the `|` literal +
indentation interactions break in surprising ways.

## ✅ DO

- One-line task bodies that invoke `bash ./scripts/<name>.sh [args…]`.
- Write the actual logic in `scripts/<name>.sh` with `set -euo pipefail`,
  proper variable quoting, and shellcheck-clean syntax.
- Pass arguments positionally: `task run RECIPE=foo` →
  `scripts/run-recipe.sh foo`.
- Keep env-var defaults in the script using the
  `VAR="${VAR:-default}"` pattern.
- Reference scripts that demonstrate the pattern:
  [`scripts/kind-up.sh`](../../scripts/kind-up.sh),
  [`scripts/run-recipe.sh`](../../scripts/run-recipe.sh),
  [`scripts/stop-server.sh`](../../scripts/stop-server.sh).

## ❌ DON'T

- **Don't** write `cmds:` blocks longer than ~2 lines.
- **Don't** use `cmds: - |` literal blocks with conditionals, loops,
  or `\` line-continued tool invocations.
- **Don't** stuff `helm upgrade --install ... \` with eight `\\`
  continuation lines into a YAML literal — extract it.
- **Don't** rely on bash idioms (`set -e`, parameter expansion
  `${VAR:-x}`, conditionals) inside YAML — go-task's mvdan/sh
  interpreter parses these but the YAML quoting story is fragile.

## Threshold rule of thumb

If the task body would need any of these, extract it:

- More than ~2 lines
- A `|` literal block scalar
- Shell idioms (`set -e`, `if`, `for`, `while`, `case`)
- Multi-line tool invocation with `\` continuations
- More than one external tool call chained with `&&`

## Example

```yaml
# ✅ DO
run:
  desc: "Run a recipe's server with hot reload (air)."
  requires:
    vars: [RECIPE]
  cmds:
    - bash ./scripts/run-recipe.sh {{.RECIPE}}
```

```yaml
# ❌ DON'T
run:
  desc: "Run a recipe's server with hot reload (air)."
  requires:
    vars: [RECIPE]
  cmds:
    - |
      air \
        --build.cmd "go build -o ./tmp/{{.RECIPE}}-server ..." \
        --build.bin "./tmp/{{.RECIPE}}-server" \
        --build.include_dir "recipes/{{.RECIPE}},pkg" \
        ... (8 more flags)
```

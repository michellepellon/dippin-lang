# Dippin for VS Code

Syntax highlighting and language support for [Dippin](https://github.com/2389-research/dippin-lang) `.dip` workflow files.

## Features

- Syntax highlighting for all Dippin constructs
- Comment toggling with `Ctrl+/` / `Cmd+/`
- Indentation-based code folding
- Auto-indent after `:` and block keywords
- Variable interpolation highlighting (`${ctx.var}`)

## Install

### From source (local install)

```sh
# Copy or symlink into VS Code extensions directory
cp -r editors/vscode ~/.vscode/extensions/dippin-lang

# Restart VS Code
```

Or create a symlink for development:

```sh
ln -s "$(pwd)/editors/vscode" ~/.vscode/extensions/dippin-lang
```

### Verify

Open any `.dip` file. The status bar should show "Dippin" as the language mode.

## Highlighted Elements

| Element | Example | Color category |
|---------|---------|---------------|
| Keywords | `workflow`, `agent`, `tool`, `human`, `edges` | keyword |
| Node names | `agent MyNode` | function |
| Field keys | `model:`, `prompt:`, `timeout:` | tag |
| Strings | `"quoted value"` | string |
| Comments | `# comment` | comment |
| Arrows | `->`, `<-` | operator |
| Conditions | `when`, `and`, `or`, `not` | keyword |
| Operators | `==`, `!=`, `<`, `>` | operator |
| Variables | `ctx.outcome`, `graph.goal` | variable |
| Interpolation | `${ctx.var}` | variable |
| Booleans | `true`, `false` | constant |
| Numbers | `3`, `60s`, `5m` | number |

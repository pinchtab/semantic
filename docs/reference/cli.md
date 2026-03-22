# CLI Reference

## Commands

### `semantic find`

Find elements matching a natural language query.

```bash
semantic find <query> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--snapshot` | stdin | Path to snapshot JSON file |
| `--threshold` | 0.3 | Minimum score |
| `--top-k` | 3 | Maximum results |
| `--strategy` | combined | `combined`, `lexical`, or `embedding` |
| `--format` | table | `table`, `json`, or `refs` |

**Examples:**

```bash
# From file
semantic find "sign in button" --snapshot page.json

# From stdin (pipe)
pinchtab snap --json | semantic find "login"
curl -s localhost:9999/snapshot | semantic find "search box"

# Machine-readable
semantic find "login" --snapshot page.json --format json

# Just refs (for piping)
semantic find "submit" --snapshot page.json --format refs
```

### `semantic match`

Score a specific element against a query.

```bash
semantic match <query> <ref> --snapshot <file>
```

**Example:**

```bash
semantic match "login button" e4 --snapshot page.json
```

### `semantic classify`

Classify a browser error string.

```bash
semantic classify <error-message>
```

**Example:**

```bash
semantic classify "could not find node with given id"
# → element_not_found (recoverable: true)

semantic classify "net::ERR_CONNECTION_REFUSED"
# → network (recoverable: false)
```

### `semantic version`

Print version information.

```bash
semantic version
```

## Snapshot Format

The CLI expects a JSON array of element descriptors:

```json
[
  {"ref": "e0", "role": "button", "name": "Sign In"},
  {"ref": "e1", "role": "textbox", "name": "Email"},
  {"ref": "e2", "role": "link", "name": "Forgot Password"}
]
```

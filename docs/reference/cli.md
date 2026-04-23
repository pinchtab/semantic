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
| `--lexical-weight` | 0 | Combined strategy lexical weight override |
| `--embedding-weight` | 0 | Combined strategy embedding weight override |
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

# Tune combined scoring
semantic find "login" --snapshot page.json --lexical-weight 0.7 --embedding-weight 0.3

# Just refs (for piping)
semantic find "submit" --snapshot page.json --format refs

# Visual layout hints
semantic find "button in top right corner" --snapshot page.json
semantic find "link below the search box" --snapshot page.json
semantic find "sidebar on the left" --snapshot page.json
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
  {
    "ref": "e0",
    "role": "button",
    "name": "Sign In",
    "interactive": true,
    "parent": "Auth card",
    "section": "Header",
    "x": 920,
    "y": 16,
    "width": 96,
    "height": 32
  },
  {
    "ref": "e1",
    "role": "textbox",
    "name": "Email",
    "positional": {
      "depth": 3,
      "sibling_index": 1,
      "sibling_count": 2,
      "labelled_by": "Email",
      "left": 120,
      "top": 240,
      "width": 320,
      "height": 36
    }
  }
]
```

Top-level geometry (`x`, `y`, `top`, `left`, `width`, `height`) and nested `positional` fields are both supported. Supplying coordinates improves results for visual hints such as `top right`, `below`, and `left`.

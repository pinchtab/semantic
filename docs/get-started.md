# Getting Started

## Install

### Go library

```bash
go get github.com/pinchtab/semantic
```

### CLI

```bash
# Via Go
go install github.com/pinchtab/semantic/cmd/semantic@latest

# Via Homebrew
brew install pinchtab/tap/semantic

# Via npm (downloads Go binary)
npm install -g @pinchtab/semantic
```

## Quick Start (Go)

```go
import "github.com/pinchtab/semantic"

elements := []semantic.ElementDescriptor{
    {Ref: "e0", Role: "button", Name: "Sign In"},
    {Ref: "e1", Role: "textbox", Name: "Email"},
    {Ref: "e2", Role: "link", Name: "Forgot password?"},
}

matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

result, err := matcher.Find(ctx, "log in button", elements, semantic.FindOptions{
    Threshold: 0.3,
    TopK:      3,
})
// result.BestRef = "e0" (synonym: "log in" → "sign in")
// result.BestScore ≈ 0.82
```

## Quick Start (CLI)

```bash
# From a snapshot file
semantic find "sign in button" --snapshot page.json

# Pipe from pinchtab
pinchtab snap --json | semantic find "login"

# Score a specific element
semantic match "login" e4 --snapshot page.json

# Classify an error
semantic classify "could not find node with given id"
```

## Development

```bash
git clone https://github.com/pinchtab/semantic
cd semantic
./dev doctor    # verify environment
./dev check     # run all checks
./dev test      # run tests
```

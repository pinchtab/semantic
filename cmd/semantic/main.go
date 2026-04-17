// Package main provides the semantic CLI tool for matching accessibility
// tree elements against natural language queries.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pinchtab/semantic"
	"github.com/pinchtab/semantic/recovery"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "find":
		runFind(os.Args[2:])
	case "match":
		runMatch(os.Args[2:])
	case "classify":
		runClassify(os.Args[2:])
	case "version":
		fmt.Println("semantic", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `semantic — accessibility element matcher

Usage:
  semantic find <query> [flags]       Find elements matching a query
  semantic match <query> <ref> [flags] Score a specific element
  semantic classify <error-message>   Classify a failure type
  semantic version                    Print version

Flags (find/match):
  --snapshot <file>   JSON snapshot file (default: stdin)
  --threshold <n>     Minimum score (default: 0.3)
  --top-k <n>         Max results (default: 3)
  --strategy <name>   lexical, embedding, or combined (default: combined)
  --format <fmt>      json, table, or refs (default: table)
`)
}

// snapshotElement is the JSON shape from pinchtab's /snapshot endpoint.
type snapshotPositional struct {
	Depth        int     `json:"depth"`
	SiblingIndex int     `json:"sibling_index"`
	SiblingCount int     `json:"sibling_count"`
	LabelledBy   string  `json:"labelled_by"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Top          float64 `json:"top"`
	Left         float64 `json:"left"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
}

type snapshotElement struct {
	Ref         string              `json:"ref"`
	Role        string              `json:"role"`
	Name        string              `json:"name"`
	Value       string              `json:"value"`
	Interactive bool                `json:"interactive"`
	Parent      string              `json:"parent"`
	Section     string              `json:"section"`
	Depth       int                 `json:"depth"`
	SiblingIdx  int                 `json:"sibling_index"`
	SiblingCnt  int                 `json:"sibling_count"`
	LabelledBy  string              `json:"labelled_by"`
	X           float64             `json:"x"`
	Y           float64             `json:"y"`
	Top         float64             `json:"top"`
	Left        float64             `json:"left"`
	Width       float64             `json:"width"`
	Height      float64             `json:"height"`
	Positional  *snapshotPositional `json:"positional"`
}

func loadSnapshot(path string) ([]semantic.ElementDescriptor, error) {
	var r io.Reader
	if path == "" || path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var elements []snapshotElement
	if err := json.Unmarshal(data, &elements); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}

	descs := make([]semantic.ElementDescriptor, len(elements))
	for i, e := range elements {
		labelledBy := e.LabelledBy
		depth := e.Depth
		siblingIdx := e.SiblingIdx
		siblingCnt := e.SiblingCnt
		x := e.X
		y := e.Y
		if x == 0 && e.Left != 0 {
			x = e.Left
		}
		if y == 0 && e.Top != 0 {
			y = e.Top
		}
		width := e.Width
		height := e.Height
		if e.Positional != nil {
			if e.Positional.Depth != 0 {
				depth = e.Positional.Depth
			}
			if e.Positional.SiblingIndex != 0 {
				siblingIdx = e.Positional.SiblingIndex
			}
			if e.Positional.SiblingCount != 0 {
				siblingCnt = e.Positional.SiblingCount
			}
			if e.Positional.LabelledBy != "" {
				labelledBy = e.Positional.LabelledBy
			}

			hasHorizontal := e.Positional.X != 0 || e.Positional.Left != 0 || e.Positional.Width > 0
			hasVertical := e.Positional.Y != 0 || e.Positional.Top != 0 || e.Positional.Height > 0
			if hasHorizontal {
				x = e.Positional.X
				if x == 0 && e.Positional.Left != 0 {
					x = e.Positional.Left
				}
				width = e.Positional.Width
			}
			if hasVertical {
				y = e.Positional.Y
				if y == 0 && e.Positional.Top != 0 {
					y = e.Positional.Top
				}
				height = e.Positional.Height
			}
		}

		descs[i] = semantic.ElementDescriptor{
			Ref:         e.Ref,
			Role:        e.Role,
			Name:        e.Name,
			Value:       e.Value,
			Interactive: e.Interactive,
			Parent:      e.Parent,
			Section:     e.Section,
			Positional: semantic.PositionalHints{
				Depth:        depth,
				SiblingIndex: siblingIdx,
				SiblingCount: siblingCnt,
				LabelledBy:   labelledBy,
				X:            x,
				Y:            y,
				Width:        width,
				Height:       height,
			},
		}
	}
	return descs, nil
}

func newMatcher(strategy string) semantic.ElementMatcher {
	switch strategy {
	case "lexical":
		return semantic.NewLexicalMatcher()
	case "embedding":
		return semantic.NewEmbeddingMatcher(semantic.NewHashingEmbedder(128))
	default:
		return semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))
	}
}

func runFind(args []string) {
	// Reorder args so flags can appear anywhere (Go's flag package
	// requires flags before positional args).
	args = reorderArgs(args)
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	snapshot := fs.String("snapshot", "", "snapshot JSON file (default: stdin)")
	threshold := fs.Float64("threshold", 0.3, "minimum score")
	topK := fs.Int("top-k", 3, "max results")
	strategy := fs.String("strategy", "combined", "matching strategy")
	format := fs.String("format", "table", "output format: json, table, refs")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: semantic find <query> [flags]")
		os.Exit(1)
	}
	query := strings.Join(fs.Args(), " ")

	elements, err := loadSnapshot(*snapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	matcher := newMatcher(*strategy)
	result, err := matcher.Find(context.Background(), query, elements, semantic.FindOptions{
		Threshold: *threshold,
		TopK:      *topK,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch *format {
	case "json":
		outputJSON(result)
	case "refs":
		outputRefs(result)
	default:
		outputTable(result)
	}
}

func runMatch(args []string) {
	args = reorderArgs(args)
	fs := flag.NewFlagSet("match", flag.ExitOnError)
	snapshot := fs.String("snapshot", "", "snapshot JSON file (default: stdin)")
	strategy := fs.String("strategy", "combined", "matching strategy")
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: semantic match <query> <ref> [flags]")
		os.Exit(1)
	}
	query := fs.Arg(0)
	targetRef := fs.Arg(1)

	elements, err := loadSnapshot(*snapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Find only the target element
	var target *semantic.ElementDescriptor
	for i := range elements {
		if elements[i].Ref == targetRef {
			target = &elements[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintf(os.Stderr, "ref %s not found in snapshot\n", targetRef)
		os.Exit(1)
	}

	matcher := newMatcher(*strategy)
	result, err := matcher.Find(context.Background(), query, []semantic.ElementDescriptor{*target}, semantic.FindOptions{
		Threshold: 0,
		TopK:      1,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(result.Matches) == 0 {
		fmt.Printf("ref=%s score=0.00 confidence=none strategy=%s\n", targetRef, result.Strategy)
	} else {
		m := result.Matches[0]
		conf := semantic.CalibrateConfidence(m.Score)
		fmt.Printf("ref=%s score=%.2f confidence=%s strategy=%s\n", m.Ref, m.Score, conf, result.Strategy)
	}
}

func runClassify(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: semantic classify <error-message>")
		os.Exit(1)
	}

	var errMsg string
	if args[0] == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		errMsg = strings.TrimSpace(string(data))
	} else {
		errMsg = strings.Join(args, " ")
	}

	ft := recovery.ClassifyFailure(fmt.Errorf("%s", errMsg))
	fmt.Printf("%s (recoverable: %v)\n", ft.String(), ft.Recoverable())
}

// reorderArgs moves flags (--key val) before positional args so Go's
// flag package can parse them regardless of where the user placed them.
func reorderArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			// Consume the next arg as the flag value if it exists and isn't a flag
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, args[i])
		}
	}
	return append(flags, positional...)
}

type jsonOutput struct {
	BestRef    string      `json:"best_ref"`
	BestScore  float64     `json:"best_score"`
	Confidence string      `json:"confidence"`
	Strategy   string      `json:"strategy"`
	Matches    []jsonMatch `json:"matches"`
}

type jsonMatch struct {
	Ref        string  `json:"ref"`
	Score      float64 `json:"score"`
	Confidence string  `json:"confidence"`
}

func outputJSON(result semantic.FindResult) {
	out := jsonOutput{
		BestRef:    result.BestRef,
		BestScore:  result.BestScore,
		Confidence: semantic.CalibrateConfidence(result.BestScore),
		Strategy:   result.Strategy,
		Matches:    make([]jsonMatch, len(result.Matches)),
	}
	for i, m := range result.Matches {
		out.Matches[i] = jsonMatch{
			Ref:        m.Ref,
			Score:      m.Score,
			Confidence: semantic.CalibrateConfidence(m.Score),
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func outputTable(result semantic.FindResult) {
	if len(result.Matches) == 0 {
		fmt.Println("No matches found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "REF\tSCORE\tCONFIDENCE\tSTRATEGY")
	for _, m := range result.Matches {
		conf := semantic.CalibrateConfidence(m.Score)
		_, _ = fmt.Fprintf(w, "%s\t%.3f\t%s\t%s\n", m.Ref, m.Score, conf, result.Strategy)
	}
	_ = w.Flush()
}

func outputRefs(result semantic.FindResult) {
	for _, m := range result.Matches {
		fmt.Println(m.Ref)
	}
}

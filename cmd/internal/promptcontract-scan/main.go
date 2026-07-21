// Command promptcontract-scan runs the in-repo schema->prompt drift detector on
// a directory and prints the drift as JSON. Operator/validation tool.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pmclSF/terrain/internal/promptcontract"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	drift, err := promptcontract.AnalyzeInRepo(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"root": root, "count": len(drift), "drift": drift})
}

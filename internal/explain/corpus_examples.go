package explain

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// Real-world OSS examples surfaced by `terrain explain` to anchor
// findings. The JSON ships embedded in the binary so there's no
// network or filesystem dependency at runtime.
//
// The embedded examples are generated from an internal public-OSS
// corpus build step; only the resulting JSON is part of the public
// distribution.

//go:embed data/corpus-examples.json
var corpusExamplesJSON []byte

// CorpusExample is one sampled firing from a public OSS repo.
type CorpusExample struct {
	Repo        string `json:"repo"`
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Explanation string `json:"explanation,omitempty"`
}

// CorpusExamplesBundle is the embedded JSON's top-level shape.
type CorpusExamplesBundle struct {
	SchemaVersion string                     `json:"schema_version"`
	GeneratedFrom string                     `json:"generated_from"`
	Examples      map[string][]CorpusExample `json:"examples"`
	SourceCounts  map[string]int             `json:"source_counts"`
}

var (
	corpusOnce    sync.Once
	corpusLoaded  *CorpusExamplesBundle
	corpusLoadErr error
)

func loadCorpusExamples() (*CorpusExamplesBundle, error) {
	corpusOnce.Do(func() {
		var b CorpusExamplesBundle
		if err := json.Unmarshal(corpusExamplesJSON, &b); err != nil {
			corpusLoadErr = err
			return
		}
		corpusLoaded = &b
	})
	return corpusLoaded, corpusLoadErr
}

// CorpusExamplesFor returns up to `max` sampled real-world firings
// for the given detector type. Returns nil if the bundle failed to
// load or the detector has no examples. Safe to call from any
// goroutine.
func CorpusExamplesFor(detectorType string, max int) []CorpusExample {
	b, err := loadCorpusExamples()
	if err != nil || b == nil {
		return nil
	}
	exs := b.Examples[detectorType]
	if max > 0 && len(exs) > max {
		exs = exs[:max]
	}
	return exs
}

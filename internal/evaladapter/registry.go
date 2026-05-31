package evaladapter

import "fmt"

// All returns every registered adapter. The order is the dispatch
// order for AutoIngest — first adapter whose CanIngest returns true
// is used. The order matches the §10 must-ship list.
//
// CanIngest checks are constructed to be mutually exclusive on real
// artifacts: promptfoo's outer `results.version` is distinct from
// deepeval's `testCases[*].metricsMetadata`, ragas's `faithfulness`
// metric keys, and GE's `expectation_config` / `evaluated_expectations`
// markers. The order matters only when an artifact happens to look
// like more than one framework — earlier wins.
func All() []Adapter {
	return []Adapter{
		PromptfooAdapter{},
		DeepevalAdapter{},
		RagasAdapter{},
		GreatExpectationsAdapter{},
	}
}

// AutoIngest tries each registered adapter in order and returns the
// first successful parse. Returns an error when no adapter recognizes
// the file. Consumers that already know the framework should call the
// adapter's Ingest directly rather than going through AutoIngest, since
// CanIngest does file I/O that's wasted on a known-framework path.
func AutoIngest(path string) (*EvalRun, error) {
	for _, a := range All() {
		if a.CanIngest(path) {
			return a.Ingest(path)
		}
	}
	return nil, fmt.Errorf("evaladapter: no registered adapter recognizes %s", path)
}

// For returns the adapter for a named framework, or nil when the
// framework isn't registered.
func For(fw Framework) Adapter {
	for _, a := range All() {
		if a.Name() == fw {
			return a
		}
	}
	return nil
}

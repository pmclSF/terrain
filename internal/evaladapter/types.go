// Package evaladapter ingests output artifacts from external eval
// frameworks (promptfoo, deepeval, ragas, Great Expectations) and
// normalizes them into a single EvalRun shape that Terrain's rules
// layer consumes.
//
// Adapters for promptfoo, deepeval, ragas, and Great Expectations are
// registered as named adapters; gauntlet artifacts are ingested
// through a JSON-compatible path rather than a named adapter. The
// shared shape lets the regression/eval-regression rule fire on any
// adopter's framework without coupling the rule to that framework's
// specific output schema.
package evaladapter

// Framework identifies one of the supported eval-framework families.
type Framework string

const (
	FrameworkPromptfoo         Framework = "promptfoo"
	FrameworkDeepeval          Framework = "deepeval"
	FrameworkRagas             Framework = "ragas"
	FrameworkGreatExpectations Framework = "great_expectations"
)

// EvalRun is the normalized shape of one complete eval-framework run.
// Adapters parse their framework's native format and produce this.
type EvalRun struct {
	// Framework identifies which framework produced the run.
	Framework Framework

	// Source is the path to the results file the run was loaded from.
	Source string

	// Timestamp is the ISO-8601 timestamp the run completed (when the
	// framework records one). Empty when the source format doesn't
	// preserve it.
	Timestamp string

	// Cases lists per-eval-case results in the order the framework
	// produced them. Order is preserved so consumers that diff runs
	// case-by-case (regression detection) can match by index.
	Cases []EvalCaseResult

	// Stats summarizes the run across all cases.
	Stats EvalRunStats
}

// EvalCaseResult is one case (single input/output pair) within an
// EvalRun. Most frameworks call this a "test case" or "row"; we call
// it a "case" to keep the noun consistent across the codebase.
type EvalCaseResult struct {
	// ID is the case identifier as recorded by the framework. May be
	// blank when the source format doesn't carry one — consumers that
	// need a stable identity fall back to ID = Name + index.
	ID string

	// Name is the human-readable case name. Falls back to a synthetic
	// `case-<index>` when the framework doesn't surface a name.
	Name string

	// Success reflects the framework's pass/fail verdict.
	Success bool

	// Score is the primary scalar metric (0–1 by convention; some
	// frameworks score on different scales — Score holds the value
	// the framework reports). For multi-metric runs, the framework's
	// declared "primary" metric is used.
	Score float64

	// Metrics carries any additional named metrics the framework
	// produced for this case (e.g., promptfoo's namedScores, ragas's
	// faithfulness/answer_relevancy/context_precision). Keys are the
	// framework's metric names; consumers that compare across runs
	// match by name.
	Metrics map[string]float64

	// Reason is the failure explanation when Success is false.
	// Frameworks vary: promptfoo's gradingResult.reason, deepeval's
	// failure_reason, ragas's score-below-threshold descriptions.
	Reason string

	// Threshold is the per-case pass threshold when the framework
	// records one. Zero when the framework uses a global threshold or
	// doesn't expose per-case thresholds.
	Threshold float64
}

// EvalRunStats aggregates the run.
type EvalRunStats struct {
	Total     int
	Successes int
	Failures  int

	// PrimaryMetric is the mean of all cases' Score values for runs
	// where every case has a Score; zero otherwise. Lets the regression
	// rule do quick run-vs-run comparison without re-aggregating.
	PrimaryMetric float64

	// HasPrimaryMetric is true when PrimaryMetric was computed from a
	// complete set of case scores (or pass rate). Distinguishes a
	// legitimate 0.0 mean from "no comparable metric available".
	HasPrimaryMetric bool
}

// Adapter reads one eval framework's output artifact and produces an
// EvalRun. Adapters are stateless — each call reads from path and
// parses fresh; no caching at this layer.
type Adapter interface {
	// Name returns the framework identifier.
	Name() Framework

	// CanIngest returns true when path looks like this adapter's
	// output format. Cheap test (filename pattern, file existence,
	// optional small read) — does not fully parse the file. Used by
	// AutoIngest to dispatch.
	CanIngest(path string) bool

	// Ingest reads and parses path into an EvalRun. Returns an error
	// when the file isn't readable or doesn't match the expected
	// schema. Adapters never panic on malformed input — they return
	// a wrapped error so consumers can degrade gracefully.
	Ingest(path string) (*EvalRun, error)
}

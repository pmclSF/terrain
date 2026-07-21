package structural

import (
	"fmt"
	"testing"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
)

// addBlastSource appends nodes/edges so AnalyzeCoverage reports the given
// (direct, indirect) test counts for srcPath. Direct tests come from a test
// file importing the source; indirect from a test file importing an
// intermediate source that imports the target.
func addBlastSource(nodes *[]*depgraph.Node, edges *[]*depgraph.Edge, srcPath string, direct, indirect int) {
	srcID := "src:" + srcPath
	*nodes = append(*nodes, &depgraph.Node{ID: srcID, Type: depgraph.NodeSourceFile, Path: srcPath})
	if direct > 0 {
		tf := "tf:" + srcPath
		*nodes = append(*nodes, &depgraph.Node{ID: tf, Type: depgraph.NodeTestFile, Path: srcPath + ".test"})
		*edges = append(*edges, &depgraph.Edge{From: tf, To: srcID, Type: depgraph.EdgeImportsModule})
		for i := 0; i < direct; i++ {
			id := fmt.Sprintf("t:%s:d%d", srcPath, i)
			*nodes = append(*nodes, &depgraph.Node{ID: id, Type: depgraph.NodeTest})
			*edges = append(*edges, &depgraph.Edge{From: id, To: tf, Type: depgraph.EdgeTestDefinedInFile})
		}
	}
	if indirect > 0 {
		mid := "src:mid:" + srcPath
		*nodes = append(*nodes, &depgraph.Node{ID: mid, Type: depgraph.NodeSourceFile, Path: "mid_" + srcPath})
		*edges = append(*edges, &depgraph.Edge{From: mid, To: srcID, Type: depgraph.EdgeSourceImportsSource})
		mtf := "tf:mid:" + srcPath
		*nodes = append(*nodes, &depgraph.Node{ID: mtf, Type: depgraph.NodeTestFile, Path: "mid_" + srcPath + ".test"})
		*edges = append(*edges, &depgraph.Edge{From: mtf, To: mid, Type: depgraph.EdgeImportsModule})
		for j := 0; j < indirect; j++ {
			id := fmt.Sprintf("t:%s:i%d", srcPath, j)
			*nodes = append(*nodes, &depgraph.Node{ID: id, Type: depgraph.NodeTest})
			*edges = append(*edges, &depgraph.Edge{From: id, To: mtf, Type: depgraph.EdgeTestDefinedInFile})
		}
	}
}

func blastSignalForFile(sigs []models.Signal, file string) *models.Signal {
	for i := range sigs {
		if sigs[i].Location.File == file {
			return &sigs[i]
		}
	}
	return nil
}

// B1: an empty graph produces no signals.
func TestBlastRadius_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := depgraph.NewGraph()
	g.Seal()
	if got := (&BlastRadiusHotspotDetector{}).DetectWithGraph(&models.TestSuiteSnapshot{}, g); len(got) != 0 {
		t.Errorf("empty graph: want 0 signals, got %d", len(got))
	}
}

// B2/B3: files below the 20-test minimum never fire.
func TestBlastRadius_BelowThreshold(t *testing.T) {
	t.Parallel()
	var n []*depgraph.Node
	var e []*depgraph.Edge
	addBlastSource(&n, &e, "a.go", 10, 0)
	addBlastSource(&n, &e, "b.go", 19, 0)
	got := (&BlastRadiusHotspotDetector{}).DetectWithGraph(&models.TestSuiteSnapshot{}, buildGraph(n, e))
	if len(got) != 0 {
		t.Errorf("all files <20 tests: want 0 signals, got %d (%+v)", len(got), got)
	}
}

// B5b: high blast radius + low direct ratio → High, with exact metadata.
func TestBlastRadius_HighSeverityLowDirectRatio(t *testing.T) {
	t.Parallel()
	var n []*depgraph.Node
	var e []*depgraph.Edge
	addBlastSource(&n, &e, "utils.go", 15, 85) // total 100, direct ratio 0.15
	got := (&BlastRadiusHotspotDetector{}).DetectWithGraph(&models.TestSuiteSnapshot{}, buildGraph(n, e))
	sig := blastSignalForFile(got, "utils.go")
	if sig == nil {
		t.Fatalf("no signal for utils.go; got %+v", got)
	}
	if sig.Severity != models.SeverityHigh {
		t.Errorf("severity: want High (total 100, ratio .15), got %v", sig.Severity)
	}
	if sig.Metadata["totalImpactedTests"] != 100 {
		t.Errorf("totalImpactedTests: want 100, got %v", sig.Metadata["totalImpactedTests"])
	}
	if sig.Metadata["directTestCount"] != 15 {
		t.Errorf("directTestCount: want 15, got %v", sig.Metadata["directTestCount"])
	}
}

// B5a: a pure-conduit barrel file (direct=0, high indirect) is demoted to Info
// instead of firing as a hotspot.
func TestBlastRadius_PureConduitDemotedToInfo(t *testing.T) {
	t.Parallel()
	var n []*depgraph.Node
	var e []*depgraph.Edge
	addBlastSource(&n, &e, "index.ts", 0, 40) // barrel re-export: 0 direct, 40 indirect
	got := (&BlastRadiusHotspotDetector{}).DetectWithGraph(&models.TestSuiteSnapshot{}, buildGraph(n, e))
	sig := blastSignalForFile(got, "index.ts")
	if sig == nil {
		t.Fatalf("no signal for index.ts; got %+v", got)
	}
	if sig.Severity != models.SeverityInfo {
		t.Errorf("pure-conduit barrel must be demoted to Info, got %v", sig.Severity)
	}
}

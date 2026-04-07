package depgraph

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// ---------------------------------------------------------------------------
// AddNode correctness
// ---------------------------------------------------------------------------

func TestAddNode_SameIDSameType_UpdatesData(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	g.AddNode(&Node{ID: "file:a.ts", Type: NodeSourceFile, Name: "old-name", Path: "a.ts"})
	g.AddNode(&Node{ID: "file:a.ts", Type: NodeSourceFile, Name: "new-name", Path: "a.ts"})

	// Node() should return the updated data.
	n := g.Node("file:a.ts")
	if n == nil || n.Name != "new-name" {
		t.Fatalf("Node() = %v, want Name=new-name", n)
	}

	// NodesByType should also return the updated data (not stale pointer).
	sources := g.NodesByType(NodeSourceFile)
	if len(sources) != 1 {
		t.Fatalf("expected 1 source node, got %d", len(sources))
	}
	if sources[0].Name != "new-name" {
		t.Errorf("NodesByType returned stale Name=%q, want new-name", sources[0].Name)
	}
}

func TestAddNode_SameIDDifferentType_ReindexesCorrectly(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	g.AddNode(&Node{ID: "file:a.ts", Type: NodeSourceFile, Name: "a.ts"})

	// Re-add with different type.
	g.AddNode(&Node{ID: "file:a.ts", Type: NodeTestFile, Name: "a.ts"})

	// Should no longer appear as source file.
	sources := g.NodesByType(NodeSourceFile)
	for _, s := range sources {
		if s.ID == "file:a.ts" {
			t.Error("node should not be in NodeSourceFile index after type change")
		}
	}

	// Should appear as test file.
	testFiles := g.NodesByType(NodeTestFile)
	found := false
	for _, tf := range testFiles {
		if tf.ID == "file:a.ts" {
			found = true
		}
	}
	if !found {
		t.Error("node should be in NodeTestFile index after type change")
	}
}

// ---------------------------------------------------------------------------
// buildSourceToSourceEdges: Phase 1 (real imports)
// ---------------------------------------------------------------------------

func TestBuildSourceToSourceEdges_Phase1_RealImports(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		ImportGraph: map[string]map[string]bool{
			"tests/auth.test.ts": {"src/auth.ts": true},
		},
		SourceImports: map[string]map[string]bool{
			"src/auth.ts": {"src/utils.ts": true, "src/db.ts": true},
		},
	}

	g := Build(snap)

	// Phase 1 should create source→source edges with high confidence.
	s2sEdges := g.EdgesByType(EdgeSourceImportsSource)
	realEdges := 0
	for _, e := range s2sEdges {
		if e.EvidenceType == EvidenceStaticAnalysis && e.Confidence == 0.95 {
			realEdges++
		}
	}
	if realEdges != 2 {
		t.Errorf("expected 2 real source-to-source edges, got %d", realEdges)
	}

	// Target nodes should be created even if not in ImportGraph.
	if g.Node("file:src/utils.ts") == nil {
		t.Error("expected src/utils.ts node created by Phase 1")
	}
	if g.Node("file:src/db.ts") == nil {
		t.Error("expected src/db.ts node created by Phase 1")
	}
}

func TestBuildSourceToSourceEdges_Phase1_DeduplicatesRealEdges(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		ImportGraph: map[string]map[string]bool{
			"tests/a.test.ts": {"src/a.ts": true},
		},
		SourceImports: map[string]map[string]bool{
			// Same edge declared twice (e.g., from different analysis passes).
			"src/a.ts": {"src/b.ts": true},
		},
	}

	g := Build(snap)

	// Should have exactly 1 source→source edge (deduplicated).
	s2s := g.EdgesByType(EdgeSourceImportsSource)
	count := 0
	for _, e := range s2s {
		if e.From == "file:src/a.ts" && e.To == "file:src/b.ts" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduplicated edge, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// buildSourceToSourceEdges: Phase 2 cross-package filter
// ---------------------------------------------------------------------------

func TestBuildSourceToSourceEdges_Phase2_CrossPackageFiltered(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		ImportGraph: map[string]map[string]bool{
			"tests/integration.test.ts": {
				"src/auth/login.ts":    true, // package: src
				"lib/utils/helpers.ts": true, // package: lib (different!)
			},
		},
	}

	g := Build(snap)

	// Phase 2 should NOT create an edge between src/auth/login.ts and
	// lib/utils/helpers.ts because they're in different packages.
	s2sEdges := g.EdgesByType(EdgeSourceImportsSource)
	for _, e := range s2sEdges {
		if e.EvidenceType == EvidenceInferred {
			t.Errorf("unexpected inferred source-to-source edge: %s → %s (cross-package should be filtered)", e.From, e.To)
		}
	}
}

func TestBuildSourceToSourceEdges_Phase2_SamePackageCreated(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		ImportGraph: map[string]map[string]bool{
			"tests/auth.test.ts": {
				"src/auth/login.ts":   true,
				"src/auth/session.ts": true,
			},
		},
	}

	g := Build(snap)

	// Phase 2 should create an inferred edge between same-package sources.
	s2sEdges := g.EdgesByType(EdgeSourceImportsSource)
	found := false
	for _, e := range s2sEdges {
		if e.EvidenceType == EvidenceInferred && e.Confidence == 0.5 {
			found = true
		}
	}
	if !found {
		t.Error("expected inferred source-to-source edge within same package")
	}
}

// ---------------------------------------------------------------------------
// Manual coverage: area resolution
// ---------------------------------------------------------------------------

func TestBuildManualCoverage_AreaGlob(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/auth/login.ts:login", Name: "login", Path: "src/auth/login.ts", Kind: models.SurfaceFunction, Package: "src/auth"},
			{SurfaceID: "surface:src/auth/logout.ts:logout", Name: "logout", Path: "src/auth/logout.ts", Kind: models.SurfaceFunction, Package: "src/auth"},
			{SurfaceID: "surface:src/billing/pay.ts:pay", Name: "pay", Path: "src/billing/pay.ts", Kind: models.SurfaceFunction, Package: "src/billing"},
		},
		ManualCoverage: []models.ManualCoverageArtifact{
			{ArtifactID: "manual:qa-auth", Name: "Auth QA", Area: "src/auth*", Source: "qa-team"},
		},
	}

	g := Build(snap)

	// The glob "src/auth*" should match auth surfaces but not billing.
	manualEdges := g.EdgesByType(EdgeManualCovers)
	matchedSurfaces := map[string]bool{}
	for _, e := range manualEdges {
		matchedSurfaces[e.To] = true
	}

	if !matchedSurfaces["surface:src/auth/login.ts:login"] {
		t.Error("expected manual coverage to match auth/login surface")
	}
	if !matchedSurfaces["surface:src/auth/logout.ts:logout"] {
		t.Error("expected manual coverage to match auth/logout surface")
	}
	if matchedSurfaces["surface:src/billing/pay.ts:pay"] {
		t.Error("manual coverage should NOT match billing surface with auth* glob")
	}
}

func TestBuildManualCoverage_ExactPackageMatch(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/auth/login.ts:login", Name: "login", Path: "src/auth/login.ts", Kind: models.SurfaceFunction, Package: "src"},
		},
		ManualCoverage: []models.ManualCoverageArtifact{
			{ArtifactID: "manual:qa", Name: "QA", Area: "src", Source: "qa"},
		},
	}

	g := Build(snap)

	// Exact package match should get confidence 0.7.
	manualEdges := g.EdgesByType(EdgeManualCovers)
	if len(manualEdges) == 0 {
		t.Fatal("expected manual coverage edge for exact package match")
	}
	if manualEdges[0].Confidence != 0.7 {
		t.Errorf("expected confidence 0.7 for exact match, got %f", manualEdges[0].Confidence)
	}
}

func TestBuildManualCoverage_AreaMatchByName(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		BehaviorSurfaces: []models.BehaviorSurface{
			{BehaviorID: "behavior:auth-flow", Label: "AuthenticationFlow", Kind: models.BehaviorGroupRoutePrefix, Package: "auth"},
		},
		ManualCoverage: []models.ManualCoverageArtifact{
			{ArtifactID: "manual:qa", Name: "QA", Area: "authentication", Source: "qa"},
		},
	}

	g := Build(snap)

	// "authentication" should prefix-match "AuthenticationFlow" (case-insensitive).
	manualEdges := g.EdgesByType(EdgeManualCovers)
	if len(manualEdges) == 0 {
		t.Error("expected manual coverage edge for name-based area match")
	}
}

// ---------------------------------------------------------------------------
// buildFixtureSurfaces: shared fixture linkage
// ---------------------------------------------------------------------------

func TestBuildFixtureSurfaces_SharedFixtureLinkedAcrossFiles(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/auth.test.ts", Framework: "jest"},
		},
		TestCases: []models.TestCase{
			{FilePath: "tests/auth.test.ts", TestName: "login", Line: 5, Framework: "jest"},
		},
		ImportGraph: map[string]map[string]bool{
			"tests/auth.test.ts": {"tests/fixtures/db.ts": true},
		},
		FixtureSurfaces: []models.FixtureSurface{
			{FixtureID: "fixture:db", Name: "dbSetup", Path: "tests/fixtures/db.ts", Kind: models.FixtureSetupHook, Shared: true},
		},
	}

	g := Build(snap)

	// Test in auth.test.ts should be linked to the shared fixture from db.ts.
	testNode := "test:tests/auth.test.ts:5:login"
	edges := g.Outgoing(testNode)
	foundFixture := false
	for _, e := range edges {
		if e.To == "fixture:db" && e.Type == EdgeTestUsesFixture {
			foundFixture = true
		}
	}
	if !foundFixture {
		t.Error("expected EdgeTestUsesFixture from test to shared fixture imported from another file")
	}
}

// ---------------------------------------------------------------------------
// buildFixtureSurfaces: fixture→surface name-based linkage
// ---------------------------------------------------------------------------

func TestBuildFixtureSurfaces_NameBasedSurfaceLinkage(t *testing.T) {
	t.Parallel()
	// Fixture and surfaces must share the same inferred package for the
	// name-based linkage to apply. inferPackage uses the first directory.
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "src/auth.test.ts", Framework: "jest"},
		},
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/auth.ts:login", Name: "login", Path: "src/auth.ts", Kind: models.SurfaceFunction, Package: "src"},
			{SurfaceID: "surface:src/auth.ts:logout", Name: "logout", Path: "src/auth.ts", Kind: models.SurfaceFunction, Package: "src"},
			{SurfaceID: "surface:lib/billing.ts:pay", Name: "pay", Path: "lib/billing.ts", Kind: models.SurfaceFunction, Package: "lib"},
		},
		FixtureSurfaces: []models.FixtureSurface{
			// Fixture in src/ package — same package as the login/logout surfaces.
			{FixtureID: "fixture:loginSetup", Name: "loginSetup", Path: "src/auth.test.ts", Kind: models.FixtureSetupHook, Language: "js"},
		},
	}

	g := Build(snap)

	// Fixture "loginSetup" contains "login" → should link to the login surface
	// (same package). Should NOT link to logout (name doesn't contain "login")
	// or pay (different package).
	fixtureEdges := g.Outgoing("fixture:loginSetup")
	linkedSurfaces := map[string]bool{}
	for _, e := range fixtureEdges {
		if e.Type == EdgeFixtureSetsSurface {
			linkedSurfaces[e.To] = true
		}
	}

	if !linkedSurfaces["surface:src/auth.ts:login"] {
		t.Error("expected fixture→surface link: loginSetup should match 'login' surface by name containment")
	}
	if linkedSurfaces["surface:lib/billing.ts:pay"] {
		t.Error("fixture should NOT link to 'pay' surface (different package)")
	}
}

// ---------------------------------------------------------------------------
// areaMatchConfidence: path exact match and glob paths
// ---------------------------------------------------------------------------

func TestAreaMatchConfidence_PathExact(t *testing.T) {
	t.Parallel()
	n := &Node{Path: "src/auth/service.ts", Package: "src/auth"}
	// Exact path match should return 0.7.
	conf := areaMatchConfidence(n, "src/auth/service.ts", false)
	if conf != 0.7 {
		t.Errorf("expected 0.7 for exact path match, got %f", conf)
	}
}

func TestAreaMatchConfidence_GlobMatch(t *testing.T) {
	t.Parallel()
	// Use a prefix that matches via path but isn't an exact package/name match.
	n := &Node{Path: "src/auth/service.ts", Package: "src/auth", Name: "AuthService"}
	conf := areaMatchConfidence(n, "src/auth/serv", true)
	if conf != 0.5 {
		t.Errorf("expected 0.5 for glob prefix match, got %f", conf)
	}
}

func TestAreaMatchConfidence_PrefixNonGlob(t *testing.T) {
	t.Parallel()
	n := &Node{Path: "src/auth/service.ts", Package: "src/auth"}
	// Non-glob prefix match should return 0.5.
	conf := areaMatchConfidence(n, "src", false)
	if conf != 0.5 {
		t.Errorf("expected 0.5 for non-glob prefix match, got %f", conf)
	}
}

// ---------------------------------------------------------------------------
// Sealed graph: caching behavior
// ---------------------------------------------------------------------------

func TestSealedGraph_NodesByTypeCached(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Type: NodeSourceFile, Name: "a"})
	g.AddNode(&Node{ID: "b", Type: NodeSourceFile, Name: "b"})
	g.Seal()

	// First call populates sortedNodes cache.
	nodes1 := g.Nodes()
	// Second call should return cached result.
	nodes2 := g.Nodes()
	if len(nodes1) != len(nodes2) {
		t.Fatalf("Nodes() returned different lengths: %d vs %d", len(nodes1), len(nodes2))
	}
}

func TestSealedGraph_NeighborsCached(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Type: NodeTestFile})
	g.AddNode(&Node{ID: "b", Type: NodeSourceFile})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeImportsModule})
	g.Seal()

	// First call computes neighbors.
	n1 := g.Neighbors("a")
	// Second call should hit cache.
	n2 := g.Neighbors("a")
	if len(n1) != 1 || len(n2) != 1 {
		t.Fatalf("Neighbors returned %d and %d, expected 1 each", len(n1), len(n2))
	}
	if n1[0] != n2[0] {
		t.Errorf("cached Neighbors mismatch: %q vs %q", n1[0], n2[0])
	}
}

func TestSealedGraph_ReverseNeighborsCached(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Type: NodeTestFile})
	g.AddNode(&Node{ID: "b", Type: NodeSourceFile})
	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeImportsModule})
	g.Seal()

	// First call computes reverse neighbors.
	r1 := g.ReverseNeighbors("b")
	// Second call should hit cache.
	r2 := g.ReverseNeighbors("b")
	if len(r1) != 1 || len(r2) != 1 {
		t.Fatalf("ReverseNeighbors returned %d and %d, expected 1 each", len(r1), len(r2))
	}
	if r1[0] != r2[0] {
		t.Errorf("cached ReverseNeighbors mismatch: %q vs %q", r1[0], r2[0])
	}
}

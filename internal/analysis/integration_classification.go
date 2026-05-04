package analysis

import (
	"context"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/testtype"
)

// refineIntegrationClassification reads each test file once via fc and
// merges any content-based integration signal (supertest, httptest,
// MockMvc, etc.) with the existing path/suite/framework-based
// classification produced by testtype.InferAll. See
// internal/testtype/integration_imports.go for the pattern allowlist
// and Track 3.3 in the 0.2.0 release plan for context.
//
// Per-file content is cached to avoid re-classifying the same file
// for every test case it contains. Cancellation via ctx is honored
// at the file-iteration boundary — important because integration
// classification runs late in the pipeline and a slow cancel here
// would still leave the user waiting.
func refineIntegrationClassification(ctx context.Context, cases []models.TestCase, fc *FileCache) []models.TestCase {
	if fc == nil || len(cases) == 0 {
		return cases
	}

	contentByPath := map[string]testtype.InferResult{}

	for i := range cases {
		if i&0x3F == 0 {
			if err := ctx.Err(); err != nil {
				return cases
			}
		}

		path := cases[i].FilePath
		if path == "" {
			continue
		}

		result, cached := contentByPath[path]
		if !cached {
			src, ok := fc.ReadFile(path)
			if !ok {
				contentByPath[path] = testtype.InferResult{Type: testtype.TypeUnknown}
				continue
			}
			result = testtype.InferFromContent(src)
			contentByPath[path] = result
		}

		if result.Type == testtype.TypeUnknown {
			continue
		}

		base := testtype.InferResult{
			Type:       cases[i].TestType,
			Confidence: cases[i].TestTypeConfidence,
			Evidence:   cases[i].TestTypeEvidence,
		}
		merged := testtype.MergeContentInference(base, result)
		cases[i].TestType = merged.Type
		cases[i].TestTypeConfidence = merged.Confidence
		cases[i].TestTypeEvidence = merged.Evidence
	}

	return cases
}

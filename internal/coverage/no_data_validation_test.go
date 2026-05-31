package coverage

import "testing"

func TestDetectNoDataValidation_FiresOnPipeline(t *testing.T) {
	t.Parallel()
	files := []DataValidationInput{
		{
			Path: "pipelines/load_users.py",
			Content: []byte(`import pandas as pd

def load():
    return pd.read_csv("users.csv")
`),
		},
	}
	sigs := DetectNoDataValidation(files)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectNoDataValidation_SuppressedByGEImport(t *testing.T) {
	t.Parallel()
	files := []DataValidationInput{
		{
			Path: "pipelines/validate.py",
			Content: []byte(`import great_expectations as ge

def validate(df): pass
`),
		},
	}
	sigs := DetectNoDataValidation(files)
	if len(sigs) != 0 {
		t.Errorf("GE import should suppress, got %+v", sigs)
	}
}

func TestDetectNoDataValidation_SuppressedByPandera(t *testing.T) {
	t.Parallel()
	files := []DataValidationInput{
		{
			Path:    "etl/transform.py",
			Content: []byte(`import pandera as pa
schema = pa.DataFrameSchema({...})
`),
		},
	}
	sigs := DetectNoDataValidation(files)
	if len(sigs) != 0 {
		t.Errorf("pandera import should suppress, got %+v", sigs)
	}
}

func TestDetectNoDataValidation_NonPipelinePathSkipped(t *testing.T) {
	t.Parallel()
	files := []DataValidationInput{
		{Path: "src/util.py", Content: []byte("def f(): return 1")},
	}
	sigs := DetectNoDataValidation(files)
	if len(sigs) != 0 {
		t.Errorf("non-pipeline path should not fire, got %+v", sigs)
	}
}

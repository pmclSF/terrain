package data

import "testing"

func TestDetectLeakageSuspected_PreprocessingBeforeSplit(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.preprocessing import StandardScaler
from sklearn.model_selection import train_test_split

scaler = StandardScaler()
X_scaled = scaler.fit_transform(X)
X_train, X_test, y_train, y_test = train_test_split(X_scaled, y, test_size=0.2)
`)
	sigs := DetectLeakageSuspected(src, "training/preprocess.py")
	found := false
	for _, s := range sigs {
		if s.Metadata["leakageKind"] == "preprocessing-leakage" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected preprocessing-leakage, got %+v", sigs)
	}
}

func TestDetectLeakageSuspected_PreprocessingAfterSplitSuppressed(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.preprocessing import StandardScaler
from sklearn.model_selection import train_test_split

X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2)

scaler = StandardScaler()
X_train_scaled = scaler.fit_transform(X_train)
X_test_scaled = scaler.transform(X_test)
`)
	sigs := DetectLeakageSuspected(src, "training/preprocess.py")
	for _, s := range sigs {
		if s.Metadata["leakageKind"] == "preprocessing-leakage" {
			t.Errorf("preprocessing-after-split should be safe, got %+v", sigs)
		}
	}
}

func TestDetectLeakageSuspected_TemporalLeakage(t *testing.T) {
	t.Parallel()
	src := []byte(`import pandas as pd
from sklearn.model_selection import train_test_split

df["date"] = pd.to_datetime(df["date"])
X_train, X_test = train_test_split(df, test_size=0.2)
`)
	sigs := DetectLeakageSuspected(src, "training/forecast.py")
	found := false
	for _, s := range sigs {
		if s.Metadata["leakageKind"] == "temporal-leakage" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected temporal-leakage, got %+v", sigs)
	}
}

func TestDetectLeakageSuspected_TimeSeriesSplitSuppressed(t *testing.T) {
	t.Parallel()
	src := []byte(`import pandas as pd
from sklearn.model_selection import TimeSeriesSplit

df["date"] = pd.to_datetime(df["date"])
tscv = TimeSeriesSplit(n_splits=5)
`)
	sigs := DetectLeakageSuspected(src, "training/forecast.py")
	for _, s := range sigs {
		if s.Metadata["leakageKind"] == "temporal-leakage" {
			t.Errorf("TimeSeriesSplit should suppress, got %+v", sigs)
		}
	}
}

func TestDetectLeakageSuspected_NonTrainingPath(t *testing.T) {
	t.Parallel()
	src := []byte(`scaler.fit_transform(X)
X_train, X_test = train_test_split(X, test_size=0.2)
`)
	sigs := DetectLeakageSuspected(src, "src/util.py")
	if len(sigs) != 0 {
		t.Errorf("non-training path should not fire, got %+v", sigs)
	}
}

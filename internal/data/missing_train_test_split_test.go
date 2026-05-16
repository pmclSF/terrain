package data

import "testing"

func TestDetectMissingTrainTestSplit_FiresOnUnsplitFit(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.ensemble import RandomForestClassifier

def train(X, y):
    clf = RandomForestClassifier()
    clf.fit(X, y)
    return clf
`)
	sigs := DetectMissingTrainTestSplit(src, "training/model.py")
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
}

func TestDetectMissingTrainTestSplit_SuppressedByTrainTestSplit(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split

def train(X, y):
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2)
    clf = RandomForestClassifier()
    clf.fit(X_train, y_train)
    return clf
`)
	sigs := DetectMissingTrainTestSplit(src, "training/model.py")
	if len(sigs) != 0 {
		t.Errorf("train_test_split should suppress, got %+v", sigs)
	}
}

func TestDetectMissingTrainTestSplit_SuppressedByKFold(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.model_selection import KFold
from sklearn.ensemble import RandomForestClassifier

def train(X, y):
    kf = KFold(n_splits=5)
    for train_idx, test_idx in kf.split(X):
        clf = RandomForestClassifier()
        clf.fit(X[train_idx], y[train_idx])
`)
	sigs := DetectMissingTrainTestSplit(src, "training/cv.py")
	if len(sigs) != 0 {
		t.Errorf("KFold should suppress, got %+v", sigs)
	}
}

func TestDetectMissingTrainTestSplit_SuppressedByCrossValScore(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.model_selection import cross_val_score
from sklearn.ensemble import RandomForestClassifier

def evaluate(X, y):
    clf = RandomForestClassifier()
    scores = cross_val_score(clf, X, y, cv=5)
    return scores
`)
	sigs := DetectMissingTrainTestSplit(src, "training/eval.py")
	if len(sigs) != 0 {
		t.Errorf("cross_val_score should suppress (no fit call anyway), got %+v", sigs)
	}
}

func TestDetectMissingTrainTestSplit_NonTrainingPath(t *testing.T) {
	t.Parallel()
	src := []byte(`def f(x, y):
    return x.fit(y)
`)
	sigs := DetectMissingTrainTestSplit(src, "src/util.py")
	if len(sigs) != 0 {
		t.Errorf("non-training path should not fire, got %+v", sigs)
	}
}

func TestDetectMissingTrainTestSplit_TimeSeriesSplit(t *testing.T) {
	t.Parallel()
	src := []byte(`from sklearn.model_selection import TimeSeriesSplit

def train(X, y):
    tscv = TimeSeriesSplit(n_splits=3)
    for tr, te in tscv.split(X):
        model.fit(X[tr], y[tr])
`)
	sigs := DetectMissingTrainTestSplit(src, "training/ts_model.py")
	if len(sigs) != 0 {
		t.Errorf("TimeSeriesSplit should suppress, got %+v", sigs)
	}
}

func TestDetectMissingTrainTestSplit_NoFit(t *testing.T) {
	t.Parallel()
	src := []byte(`def preprocess(X): return X * 2
`)
	sigs := DetectMissingTrainTestSplit(src, "training/preprocess.py")
	if len(sigs) != 0 {
		t.Errorf("no fit call should not fire, got %+v", sigs)
	}
}

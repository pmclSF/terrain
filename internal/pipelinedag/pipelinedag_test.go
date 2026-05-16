package pipelinedag

import (
	"testing"
)

func TestDetectPipelines_AirflowDAGAssignment(t *testing.T) {
	t.Parallel()
	src := []byte(`
from airflow import DAG
from airflow.operators.python import PythonOperator
from datetime import datetime

dag = DAG(
    "daily_summary",
    schedule="0 1 * * *",
    start_date=datetime(2024, 1, 1),
)

extract_task = PythonOperator(
    task_id="extract",
    python_callable=lambda: None,
    dag=dag,
)

transform_task = PythonOperator(
    task_id="transform",
    python_callable=lambda: None,
    dag=dag,
)
`)
	got := DetectPipelines(src, "dags/daily.py")
	if len(got) != 1 {
		t.Fatalf("pipelines = %d, want 1: %+v", len(got), got)
	}
	p := got[0]
	if p.Framework != "airflow" {
		t.Errorf("framework = %q", p.Framework)
	}
	if p.Name != "daily_summary" {
		t.Errorf("name = %q", p.Name)
	}
	if len(p.Tasks) != 2 {
		t.Errorf("tasks = %v, want 2 (extract, transform)", p.Tasks)
	}
}

func TestDetectPipelines_AirflowWithStatement(t *testing.T) {
	t.Parallel()
	src := []byte(`
from airflow import DAG
from airflow.operators.bash import BashOperator

with DAG("nightly_etl", schedule="@daily") as dag:
    cleanup = BashOperator(task_id="cleanup", bash_command="rm -rf /tmp/old")
`)
	got := DetectPipelines(src, "dags/nightly.py")
	if len(got) != 1 {
		t.Fatalf("pipelines = %d, want 1: %+v", len(got), got)
	}
	if got[0].Name != "nightly_etl" {
		t.Errorf("name = %q", got[0].Name)
	}
	if len(got[0].Tasks) != 1 || got[0].Tasks[0] != "cleanup" {
		t.Errorf("tasks = %v", got[0].Tasks)
	}
}

func TestDetectPipelines_AirflowDecorator(t *testing.T) {
	t.Parallel()
	src := []byte(`
from airflow.decorators import dag, task
from datetime import datetime

@dag(dag_id="modern_dag", schedule="@hourly", start_date=datetime(2024, 1, 1))
def my_pipeline():
    @task
    def step_one():
        return 1

    @task
    def step_two():
        return 2

    step_one() >> step_two()
`)
	got := DetectPipelines(src, "dags/modern.py")
	if len(got) != 1 {
		t.Fatalf("pipelines = %d, want 1: %+v", len(got), got)
	}
	if got[0].Name != "modern_dag" {
		t.Errorf("name = %q", got[0].Name)
	}
	if len(got[0].Tasks) != 2 {
		t.Errorf("tasks = %v, want 2", got[0].Tasks)
	}
}

func TestDetectPipelines_PrefectFlow(t *testing.T) {
	t.Parallel()
	src := []byte(`
from prefect import flow, task

@task
def fetch():
    return [1, 2, 3]

@task
def process(data):
    return sum(data)

@flow(name="data_aggregation")
def main():
    data = fetch()
    return process(data)
`)
	got := DetectPipelines(src, "flows/agg.py")
	if len(got) != 1 {
		t.Fatalf("pipelines = %d, want 1: %+v", len(got), got)
	}
	if got[0].Framework != "prefect" {
		t.Errorf("framework = %q", got[0].Framework)
	}
	if got[0].Name != "data_aggregation" {
		t.Errorf("name = %q", got[0].Name)
	}
	if len(got[0].Tasks) != 2 {
		t.Errorf("tasks = %v, want 2", got[0].Tasks)
	}
}

func TestDetectPipelines_PrefectBareDecorator(t *testing.T) {
	t.Parallel()
	src := []byte(`
from prefect import flow

@flow
def my_workflow():
    return "ok"
`)
	got := DetectPipelines(src, "flows/bare.py")
	if len(got) != 1 {
		t.Fatalf("pipelines = %d, want 1: %+v", len(got), got)
	}
	// Bare @flow uses the function name.
	if got[0].Name != "my_workflow" {
		t.Errorf("name = %q, want my_workflow (function-name fallback)", got[0].Name)
	}
}

func TestDetectPipelines_NoFramework(t *testing.T) {
	t.Parallel()
	src := []byte(`
import os
import json

def main():
    pass
`)
	if got := DetectPipelines(src, "x.py"); got != nil {
		t.Errorf("expected nil for non-pipeline source, got %+v", got)
	}
}

func TestDetectPipelines_Empty(t *testing.T) {
	t.Parallel()
	if got := DetectPipelines(nil, "x.py"); got != nil {
		t.Errorf("nil source: %+v", got)
	}
	if got := DetectPipelines([]byte(""), "x.py"); got != nil {
		t.Errorf("empty source: %+v", got)
	}
}

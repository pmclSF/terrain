// Package pipelinedag parses Python source for Airflow DAGs and
// Prefect flows. Both frameworks express orchestration via Python
// decorators / class instantiation; per-file parsing surfaces the
// DAG ID / flow name and contained tasks, which the unified graph
// uses as pipeline-level nodes alongside dbt manifest entries.
//
// Detection is AST-based via tree-sitter Python, using the same
// parser-pool pattern as internal/aidetect/ast_python.go. Returns
// nil when parsing fails so callers degrade gracefully.
package pipelinedag

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// Pipeline is one parsed DAG/flow definition.
type Pipeline struct {
	// Framework identifies the source — "airflow" or "prefect".
	Framework string

	// Name is the DAG ID (Airflow) or flow name (Prefect). Falls back
	// to the Python identifier when the framework allows naming
	// shorthand (e.g., @flow without args uses the function name).
	Name string

	// Path is the source file path (set by the caller; the parser
	// itself only sees the source bytes).
	Path string

	// Line is the 1-based line number where the DAG/flow declaration
	// starts.
	Line int

	// Tasks lists task names defined within this pipeline. Airflow:
	// PythonOperator(task_id="x", ...) or @task def x(): ...
	// Prefect: @task def x(): ... (in scope of the @flow).
	Tasks []string
}

// DetectPipelines parses a Python source buffer and returns one
// Pipeline per detected DAG or flow.
func DetectPipelines(src []byte, relPath string) []Pipeline {
	if len(src) == 0 {
		return nil
	}

	var pipelines []Pipeline
	var parseOK bool

	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()

		imports := collectPipelineImports(tree.RootNode(), src)
		if imports == "" {
			return nil // file doesn't import a pipeline framework
		}

		walkForPipelines(tree.RootNode(), src, imports, relPath, &pipelines)
		// Build task list per pipeline by walking the same tree.
		tasks := collectTasks(tree.RootNode(), src, imports)
		for i := range pipelines {
			pipelines[i].Tasks = tasks
		}
		parseOK = true
		return nil
	})

	if !parseOK {
		return nil
	}
	return pipelines
}

// collectPipelineImports returns "airflow" or "prefect" when the file
// imports the corresponding framework, or "" otherwise.
func collectPipelineImports(root *sitter.Node, src []byte) string {
	var found string

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil || found != "" {
			return
		}
		switch n.Type() {
		case "import_statement", "import_from_statement":
			text := nodeText(n, src)
			switch {
			case strings.Contains(text, "airflow"):
				found = "airflow"
			case strings.Contains(text, "prefect"):
				found = "prefect"
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return found
}

// walkForPipelines finds DAG/flow declarations.
//
// Airflow patterns:
//   - dag = DAG("my_id", ...)            (call_expression assignment)
//   - with DAG("my_id", ...) as dag:     (with_statement)
//   - @dag(dag_id="my_id") def f():      (decorated function definition)
//
// Prefect patterns:
//   - @flow def f():                      (decorated function definition)
//   - @flow(name="my_name") def f():     (decorated function with kwargs)
func walkForPipelines(node *sitter.Node, src []byte, framework, relPath string, out *[]Pipeline) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "assignment":
		// dag = DAG("...", ...)
		valueNode := node.ChildByFieldName("right")
		if valueNode != nil && valueNode.Type() == "call" {
			if p := pipelineFromCall(valueNode, src, framework); p != nil {
				p.Path = relPath
				p.Line = int(node.StartPoint().Row) + 1
				*out = append(*out, *p)
			}
		}
	case "with_statement":
		// with DAG("...", ...) as dag:
		walkWithStatement(node, src, framework, relPath, out)
	case "decorated_definition":
		// @dag / @flow decorators on def
		if p := pipelineFromDecorated(node, src, framework); p != nil {
			p.Path = relPath
			p.Line = int(node.StartPoint().Row) + 1
			*out = append(*out, *p)
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkForPipelines(node.Child(i), src, framework, relPath, out)
	}
}

// pipelineFromCall handles `DAG("my_id", ...)` and similar.
func pipelineFromCall(call *sitter.Node, src []byte, framework string) *Pipeline {
	funcNode := call.ChildByFieldName("function")
	if funcNode == nil {
		return nil
	}
	name := nodeText(funcNode, src)
	if !isPipelineCallable(name, framework) {
		return nil
	}
	argsNode := call.ChildByFieldName("arguments")
	id := extractPipelineID(argsNode, src, framework)
	return &Pipeline{Framework: framework, Name: id}
}

// pipelineFromDecorated handles `@dag` / `@flow` on def.
func pipelineFromDecorated(decorated *sitter.Node, src []byte, framework string) *Pipeline {
	// decorated_definition has two child kinds: decorator(s) + definition.
	var decoratorName string
	var decoratorCall *sitter.Node
	var funcName string

	for i := 0; i < int(decorated.NamedChildCount()); i++ {
		child := decorated.NamedChild(i)
		switch child.Type() {
		case "decorator":
			// decorator's inner content is either a call_expression
			// (@dag(args)) or a bare identifier/attribute (@flow).
			if child.NamedChildCount() > 0 {
				inner := child.NamedChild(0)
				if inner.Type() == "call" {
					decoratorCall = inner
					if fn := inner.ChildByFieldName("function"); fn != nil {
						decoratorName = nodeText(fn, src)
					}
				} else {
					decoratorName = nodeText(inner, src)
				}
			}
		case "function_definition":
			if name := child.ChildByFieldName("name"); name != nil {
				funcName = nodeText(name, src)
			}
		}
	}

	if !isPipelineDecorator(decoratorName, framework) {
		return nil
	}

	id := ""
	if decoratorCall != nil {
		argsNode := decoratorCall.ChildByFieldName("arguments")
		id = extractPipelineID(argsNode, src, framework)
	}
	if id == "" {
		id = funcName
	}
	return &Pipeline{Framework: framework, Name: id}
}

// walkWithStatement handles `with DAG("id", ...) as dag:`.
//
// The Python grammar nests the call inside a with_item which may
// itself wrap an as_pattern when the `as <name>` form is used.
// We look for the first call descendant within the with_statement's
// header; the rest (suite body) is handled by the normal walker.
func walkWithStatement(withStmt *sitter.Node, src []byte, framework, relPath string, out *[]Pipeline) {
	// Find the first `:` to bound our search to the header.
	headerEnd := withStmt.EndByte()
	for i := 0; i < int(withStmt.ChildCount()); i++ {
		child := withStmt.Child(i)
		if child.Type() == ":" {
			headerEnd = child.StartByte()
			break
		}
	}

	var found *sitter.Node
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil || found != nil {
			return
		}
		if n.StartByte() >= headerEnd {
			return
		}
		if n.Type() == "call" {
			found = n
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(withStmt)

	if found != nil {
		if p := pipelineFromCall(found, src, framework); p != nil {
			p.Path = relPath
			p.Line = int(withStmt.StartPoint().Row) + 1
			*out = append(*out, *p)
		}
	}
}

// isPipelineCallable returns true when the called name looks like a
// DAG/flow constructor for the given framework.
func isPipelineCallable(name, framework string) bool {
	switch framework {
	case "airflow":
		// `DAG("...")` or `airflow.DAG(...)` or `models.DAG(...)`.
		return name == "DAG" || strings.HasSuffix(name, ".DAG")
	case "prefect":
		return name == "Flow" || strings.HasSuffix(name, ".Flow")
	}
	return false
}

// isPipelineDecorator returns true when the decorator name matches the
// framework's DAG/flow decorator.
func isPipelineDecorator(name, framework string) bool {
	switch framework {
	case "airflow":
		return name == "dag" || strings.HasSuffix(name, ".dag")
	case "prefect":
		return name == "flow" || strings.HasSuffix(name, ".flow")
	}
	return false
}

// extractPipelineID pulls the DAG ID / flow name from a call's arguments.
//
// Airflow `DAG("my_id", ...)` — first positional argument is the ID.
// Airflow `DAG(dag_id="my_id", ...)` — `dag_id` keyword.
// Prefect `@flow(name="my_flow")` — `name` keyword.
func extractPipelineID(argsNode *sitter.Node, src []byte, framework string) string {
	if argsNode == nil {
		return ""
	}
	idKeyword := ""
	switch framework {
	case "airflow":
		idKeyword = "dag_id"
	case "prefect":
		idKeyword = "name"
	}

	// First pass: look for the keyword argument.
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() != "keyword_argument" {
			continue
		}
		nameNode := arg.ChildByFieldName("name")
		valueNode := arg.ChildByFieldName("value")
		if nameNode == nil || valueNode == nil {
			continue
		}
		if nodeText(nameNode, src) == idKeyword && valueNode.Type() == "string" {
			return stripStringQuotes(nodeText(valueNode, src))
		}
	}

	// Second pass: first positional string arg (Airflow shorthand).
	if framework == "airflow" {
		for i := 0; i < int(argsNode.NamedChildCount()); i++ {
			arg := argsNode.NamedChild(i)
			if arg.Type() == "string" {
				return stripStringQuotes(nodeText(arg, src))
			}
		}
	}
	return ""
}

// collectTasks walks the tree for task names visible to a single
// DAG/flow. This is a heuristic — accurate task-to-DAG association
// requires scope analysis; for now we treat the file as having one
// DAG/flow and surface all detected tasks under it.
func collectTasks(root *sitter.Node, src []byte, framework string) []string {
	var tasks []string
	seen := map[string]bool{}

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "decorated_definition":
			task := taskFromDecorated(n, src)
			if task != "" && !seen[task] {
				seen[task] = true
				tasks = append(tasks, task)
			}
		case "call":
			// Airflow: PythonOperator(task_id="my_task", ...)
			if framework == "airflow" {
				task := taskFromOperatorCall(n, src)
				if task != "" && !seen[task] {
					seen[task] = true
					tasks = append(tasks, task)
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return tasks
}

func taskFromDecorated(decorated *sitter.Node, src []byte) string {
	var isTask bool
	var funcName string
	for i := 0; i < int(decorated.NamedChildCount()); i++ {
		child := decorated.NamedChild(i)
		switch child.Type() {
		case "decorator":
			if child.NamedChildCount() > 0 {
				inner := child.NamedChild(0)
				name := ""
				if inner.Type() == "call" {
					if fn := inner.ChildByFieldName("function"); fn != nil {
						name = nodeText(fn, src)
					}
				} else {
					name = nodeText(inner, src)
				}
				if name == "task" || strings.HasSuffix(name, ".task") {
					isTask = true
				}
			}
		case "function_definition":
			if name := child.ChildByFieldName("name"); name != nil {
				funcName = nodeText(name, src)
			}
		}
	}
	if isTask {
		return funcName
	}
	return ""
}

func taskFromOperatorCall(call *sitter.Node, src []byte) string {
	funcNode := call.ChildByFieldName("function")
	if funcNode == nil {
		return ""
	}
	name := nodeText(funcNode, src)
	// Operator-style class names end in "Operator".
	rootName := name
	if i := strings.LastIndex(name, "."); i >= 0 {
		rootName = name[i+1:]
	}
	if !strings.HasSuffix(rootName, "Operator") {
		return ""
	}
	argsNode := call.ChildByFieldName("arguments")
	if argsNode == nil {
		return ""
	}
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() != "keyword_argument" {
			continue
		}
		nameNode := arg.ChildByFieldName("name")
		valueNode := arg.ChildByFieldName("value")
		if nameNode == nil || valueNode == nil {
			continue
		}
		if nodeText(nameNode, src) == "task_id" && valueNode.Type() == "string" {
			return stripStringQuotes(nodeText(valueNode, src))
		}
	}
	return ""
}

func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}

func stripStringQuotes(s string) string {
	s = strings.TrimSpace(s)
	// Drop a leading run of Python string-prefix letters (f/r/b/u, any
	// case and combination) so prefixed literals like f"etl_dag" or
	// r'raw_id' strip down to their inner text rather than keeping the
	// prefix and quotes.
	for len(s) > 0 {
		c := s[0]
		if c == 'f' || c == 'F' || c == 'r' || c == 'R' ||
			c == 'b' || c == 'B' || c == 'u' || c == 'U' {
			s = s[1:]
			continue
		}
		break
	}
	if len(s) < 2 {
		return s
	}
	first := s[0]
	last := s[len(s)-1]
	if (first == '"' || first == '\'') && first == last {
		return s[1 : len(s)-1]
	}
	return s
}

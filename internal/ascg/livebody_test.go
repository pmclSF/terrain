package ascg

import "testing"

// TestIsLiveCodeBody exercises the FileBody structural override that
// flips a catalog-path verdict to Live when the body contains real
// code shapes. Locking these cases in keeps the override from
// regressing on the "examples/server.py" class of finding.
func TestIsLiveCodeBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "python def + openai call",
			body: "from openai import OpenAI\n\nclient = OpenAI()\n\ndef chat(prompt):\n    return client.chat.completions.create(model='gpt-4', messages=[{'role':'user','content':prompt}])\n",
			want: true,
		},
		{
			name: "python def + langchain import (no call)",
			body: "from langchain.chains import RetrievalQA\n\ndef build_chain():\n    pass\n",
			want: true,
		},
		{
			name: "typescript class + sdk call",
			body: "import OpenAI from 'openai'\n\nexport class Agent {\n  async run(): Promise<string> {\n    const c = new OpenAI()\n    return (await c.chat.completions.create({model:'gpt-4',messages:[]})).choices[0].message.content\n  }\n}\n",
			want: true,
		},
		{
			name: "go func + http call",
			body: "package main\n\nimport \"net/http\"\n\nfunc main() {\n  http.Get(\"https://example.com\")\n}\n",
			want: true,
		},
		{
			name: "readme prose only",
			body: "# Title\n\nThis example shows how to call OpenAI. See examples/server.py.\n",
			want: false,
		},
		{
			name: "data dict only",
			body: "MODELS = {\n  'gpt-4': {'context': 8192},\n  'gpt-3.5-turbo': {'context': 4096},\n}\n",
			want: false,
		},
		{
			name: "empty",
			body: "",
			want: false,
		},
		{
			name: "class def without call or SDK import",
			body: "class Foo:\n    pass\n",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLiveCodeBody(tc.body)
			if got != tc.want {
				t.Errorf("isLiveCodeBody(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestClassify_FileBodyOverridesCatalogPath asserts the end-to-end
// override: when the path looks catalog-shaped (examples/) but the
// body contains real code shapes, Classify flips to Live.
func TestClassify_FileBodyOverridesCatalogPath(t *testing.T) {
	liveBody := "from openai import OpenAI\nclient = OpenAI()\ndef chat(p):\n  return client.chat.completions.create(model='gpt-4', messages=[])\n"
	noteBody := "# This is just a README.\nSee examples/server.py.\n"

	t.Run("examples/server.py with real code -> Live", func(t *testing.T) {
		r := Classify(Location{Path: "examples/server.py", FileBody: liveBody})
		if r.Class != Live {
			t.Errorf("Class = %v, want Live (catalog path should be overridden by live body); reasons=%v", r.Class, r.Reasons)
		}
		hasOverride := false
		for _, reason := range r.Reasons {
			if reason == "live_code_body_structural" {
				hasOverride = true
			}
		}
		if !hasOverride {
			t.Errorf("expected 'live_code_body_structural' reason, got %v", r.Reasons)
		}
	})

	t.Run("examples/README.md with prose -> stays Catalog", func(t *testing.T) {
		r := Classify(Location{Path: "examples/README.md", FileBody: noteBody})
		if r.Class != CatalogOrExample {
			t.Errorf("Class = %v, want CatalogOrExample (markdown stays catalog regardless of body); reasons=%v", r.Class, r.Reasons)
		}
	})

	t.Run("examples/server.py with prose-only body -> stays Catalog", func(t *testing.T) {
		r := Classify(Location{Path: "examples/server.py", FileBody: noteBody})
		if r.Class != CatalogOrExample {
			t.Errorf("Class = %v, want CatalogOrExample (no live-code shape in body); reasons=%v", r.Class, r.Reasons)
		}
	})

	t.Run("examples/server.py with empty body -> stays Catalog", func(t *testing.T) {
		r := Classify(Location{Path: "examples/server.py", FileBody: ""})
		if r.Class != CatalogOrExample {
			t.Errorf("Class = %v, want CatalogOrExample (no body = no override); reasons=%v", r.Class, r.Reasons)
		}
	})
}

package convert

import "testing"

func BenchmarkConvertCypressToPlaywrightSource_ASTPath(b *testing.B) {
	source, ok := legacySourceFixture("cypress")
	if !ok {
		b.Fatal("missing cypress fixture")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ConvertCypressToPlaywrightSource(source); err != nil {
			b.Fatalf("ConvertCypressToPlaywrightSource returned error: %v", err)
		}
	}
}

func BenchmarkConvertPuppeteerToPlaywrightSource_FallbackPath(b *testing.B) {
	fixture, ok := malformedJSFixture("puppeteer")
	if !ok {
		b.Fatal("missing puppeteer malformed fixture")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ConvertPuppeteerToPlaywrightSource(fixture.input); err != nil {
			b.Fatalf("ConvertPuppeteerToPlaywrightSource returned error: %v", err)
		}
	}
}

func BenchmarkConvertJestToVitestSource_FallbackPath(b *testing.B) {
	fixture, ok := malformedJSFixture("jest")
	if !ok {
		b.Fatal("missing jest malformed fixture")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ConvertJestToVitestSource(fixture.input); err != nil {
			b.Fatalf("ConvertJestToVitestSource returned error: %v", err)
		}
	}
}

func BenchmarkConvertPlaywrightToCypressSource_ASTPath(b *testing.B) {
	source, ok := legacySourceFixture("playwright")
	if !ok {
		b.Fatal("missing playwright fixture")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ConvertPlaywrightToCypressSource(source); err != nil {
			b.Fatalf("ConvertPlaywrightToCypressSource returned error: %v", err)
		}
	}
}

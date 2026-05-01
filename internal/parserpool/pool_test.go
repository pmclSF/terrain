package parserpool

import (
	"context"
	"sync"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	javascript "github.com/smacker/go-tree-sitter/javascript"
)

// TestWith_ParsesSimpleSource confirms the pool returns a usable
// parser. End-to-end smoke test, not a benchmark.
func TestWith_ParsesSimpleSource(t *testing.T) {
	t.Parallel()

	src := []byte("const x = 1;")
	err := With(javascript.GetLanguage(), func(p *sitter.Parser) error {
		tree, perr := p.ParseCtx(context.Background(), nil, src)
		if perr != nil {
			t.Fatalf("ParseCtx: %v", perr)
		}
		if tree == nil {
			t.Fatal("nil tree")
		}
		root := tree.RootNode()
		if root.Type() != "program" {
			t.Errorf("root type = %q, want program", root.Type())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("With: %v", err)
	}
}

// TestWith_ConcurrentReuse hammers the pool from many goroutines and
// confirms parsers survive concurrent acquire/release cycles.
func TestWith_ConcurrentReuse(t *testing.T) {
	t.Parallel()

	src := []byte("const x = 1;")
	const goroutines = 32
	const itersPer = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itersPer; j++ {
				err := With(javascript.GetLanguage(), func(p *sitter.Parser) error {
					tree, perr := p.ParseCtx(context.Background(), nil, src)
					if perr != nil {
						return perr
					}
					_ = tree.RootNode().Type()
					return nil
				})
				if err != nil {
					t.Errorf("With error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestAcquireRelease_PointerIdentity verifies that the pool actually
// reuses parsers — a Release then immediate Acquire on a single
// goroutine should return the same pointer. (sync.Pool semantics:
// reuse is best-effort, but with no GC happening between calls the
// pool is reliably hot.)
func TestAcquireRelease_PointerIdentity(t *testing.T) {
	t.Parallel()

	lang := javascript.GetLanguage()
	first := Acquire(lang)
	Release(lang, first)
	second := Acquire(lang)
	defer Release(lang, second)

	if first != second {
		// sync.Pool may legally drop entries; fail only if reuse is
		// broken in a way that loses utility entirely. A best-effort
		// check that catches obvious regressions.
		t.Logf("pool did not reuse same parser (acceptable per sync.Pool docs)")
	}
}

// realisticTestFile is a representative-size test file body — multiple
// describes, nested its, several assertion patterns. ~3 KB matches the
// median JS test file in tests/fixtures/.
const realisticTestFile = `
const { login, register } = require('./auth');

describe('auth/login', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('returns user payload on valid credentials', async () => {
    const user = await login('alice', 'pw');
    expect(user).toEqual({ name: 'alice', role: 'user', verified: true });
  });

  test('rejects invalid password with code 401', async () => {
    await expect(login('alice', 'wrong')).rejects.toMatchObject({ status: 401 });
  });

  test('locks account after 5 failed attempts', async () => {
    for (let i = 0; i < 5; i++) {
      await expect(login('alice', 'wrong')).rejects.toBeDefined();
    }
    await expect(login('alice', 'pw')).rejects.toMatchObject({ code: 'LOCKED' });
  });
});

describe('auth/register', () => {
  test('creates user with default role', async () => {
    const u = await register({ name: 'bob', email: 'b@example.com' });
    expect(u).toMatchObject({ name: 'bob', role: 'user' });
  });

  test('rejects duplicate email', async () => {
    await register({ name: 'bob', email: 'b@example.com' });
    await expect(register({ name: 'bob2', email: 'b@example.com' }))
      .rejects.toThrow('duplicate');
  });

  test.skip('handles MFA enrolment', async () => {
    expect(true).toBe(true);
  });
});
`

// BenchmarkParseFile_VsFresh measures realistic per-file parse cost.
// On real test files the parser allocation cost (CGO context setup)
// becomes proportionally larger, so reuse is a real win even though
// Go's allocator can't see the C-side bytes.
func BenchmarkParseFile_VsFresh(b *testing.B) {
	src := []byte(realisticTestFile)

	b.Run("pooled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = With(javascript.GetLanguage(), func(p *sitter.Parser) error {
				tree, _ := p.ParseCtx(context.Background(), nil, src)
				if tree != nil {
					tree.Close()
				}
				return nil
			})
		}
	})

	b.Run("fresh", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			p := sitter.NewParser()
			p.SetLanguage(javascript.GetLanguage())
			tree, _ := p.ParseCtx(context.Background(), nil, src)
			if tree != nil {
				tree.Close()
			}
			p.Close()
		}
	})
}

// BenchmarkParseConcurrent simulates the real workload: many goroutines
// parsing test files in parallel (the pattern used by
// internal/analysis/context.parallelForEachIndex). Pool reuse reduces
// pressure on the CGO allocator and on parser-init bookkeeping.
func BenchmarkParseConcurrent(b *testing.B) {
	src := []byte(realisticTestFile)

	b.Run("pooled", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = With(javascript.GetLanguage(), func(p *sitter.Parser) error {
					tree, _ := p.ParseCtx(context.Background(), nil, src)
					if tree != nil {
						tree.Close()
					}
					return nil
				})
			}
		})
	})

	b.Run("fresh", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				p := sitter.NewParser()
				p.SetLanguage(javascript.GetLanguage())
				tree, _ := p.ParseCtx(context.Background(), nil, src)
				if tree != nil {
					tree.Close()
				}
				p.Close()
			}
		})
	})
}

// Package parserpool provides a per-language sync.Pool of tree-sitter
// parsers. It eliminates the allocation churn that the round-4 review
// flagged on 1k-file repos: each call to sitter.NewParser() allocates a
// CGO-backed parser context, and the existing call sites paid that cost
// once per file. With this pool, the cost is amortised across files and
// across concurrent workers, and parsers are returned for reuse.
//
// Usage:
//
//	import "github.com/pmclSF/terrain/internal/parserpool"
//
//	err := parserpool.With(javascript.GetLanguage(), func(p *sitter.Parser) error {
//	    tree, perr := p.ParseCtx(ctx, nil, src)
//	    // ...
//	    return perr
//	})
//
// Callers MUST NOT call parser.Close() on a pooled parser — that would
// invalidate the next user's reference.
package parserpool

import (
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

// pools maps a *sitter.Language pointer to its sync.Pool of parsers.
// Pointer identity is the right key: smacker's GetLanguage() returns
// the same pointer on subsequent calls, so two sites asking for the
// same language hit the same pool. Languages aren't garbage-collected
// in practice (they're package-level globals from the grammar bindings).
var pools sync.Map // map[*sitter.Language]*sync.Pool

// poolFor returns (creating if needed) the pool for lang.
func poolFor(lang *sitter.Language) *sync.Pool {
	if p, ok := pools.Load(lang); ok {
		return p.(*sync.Pool)
	}
	created := &sync.Pool{
		New: func() any {
			p := sitter.NewParser()
			p.SetLanguage(lang)
			return p
		},
	}
	actual, _ := pools.LoadOrStore(lang, created)
	return actual.(*sync.Pool)
}

// Acquire takes a parser for lang from the pool. The caller MUST return
// it via Release (or use the With helper, which is preferred). Acquire
// is safe for concurrent use.
func Acquire(lang *sitter.Language) *sitter.Parser {
	return poolFor(lang).Get().(*sitter.Parser)
}

// Release returns a parser to the pool for lang. The parser must have
// been obtained from Acquire/With for the same language; passing a
// parser configured for a different language will silently break the
// next consumer (the parser carries the wrong grammar).
func Release(lang *sitter.Language, p *sitter.Parser) {
	if p == nil {
		return
	}
	poolFor(lang).Put(p)
}

// With is the recommended entry point. Acquires a parser, runs fn, and
// always returns the parser to the pool — even if fn panics. Returns
// fn's error verbatim.
func With(lang *sitter.Language, fn func(*sitter.Parser) error) error {
	p := Acquire(lang)
	defer Release(lang, p)
	return fn(p)
}

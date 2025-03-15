package cypher

import "testing"

func TestOptimizedCacheReuse(t *testing.T) {
	cache := NewOptimizedCache()
	node := &LiteralData{Value: "cache_test"}

	buildCount := 0
	compileFn := func() string {
		buildCount++
		c := NewCompiler()
		c.Compile(node)
		return c.Output()
	}

	out1 := cache.Fetch(node, compileFn)
	out2 := cache.Fetch(node, compileFn)

	if buildCount != 1 {
		t.Fatalf("expected compileFn to run once, ran %d times", buildCount)
	}

	if out1 != "$p1" || out2 != "$p1" {
		t.Fatalf("expected cached output '$p1', got %q and %q", out1, out2)
	}
}

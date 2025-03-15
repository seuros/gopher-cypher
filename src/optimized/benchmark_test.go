package optimized_test

import (
	"testing"

	"github.com/seuros/gopher-cypher/src/cypher"
	optimizedpkg "github.com/seuros/gopher-cypher/src/optimized"
)

func BenchmarkLiteralData(b *testing.B) {
	b.ReportAllocs()
	node := &cypher.ReturnNode{Items: []interface{}{&optimizedpkg.LiteralData{Value: 1}}}
	for i := 0; i < b.N; i++ {
		c := cypher.NewCompiler()
		c.Compile(node)
	}
}

func BenchmarkCacheFetch(b *testing.B) {
	b.ReportAllocs()
	c := optimizedpkg.NewCache()
	n := &optimizedpkg.LiteralData{Value: 42}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Fetch(n, func() string { return "compiled" })
	}
}

// BenchmarkCacheFetchMiss measures the overhead of inserting new entries
// into the cache. A fresh node is used on each iteration so the lookup
// always results in a miss and triggers an insert.
func BenchmarkCacheFetchMiss(b *testing.B) {
	c := optimizedpkg.NewCache()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := &optimizedpkg.LiteralData{Value: i}
		c.Fetch(n, func() string { return "compiled" })
	}
}

// BenchmarkCacheEvict pre-fills the cache to its maximum size and then
// measures the cost of inserting one additional entry which forces an
// eviction on every iteration.
func BenchmarkCacheEvict(b *testing.B) {
	c := optimizedpkg.NewCache()

	// Pre-fill the cache to maxSize so it is full before the benchmark
	// loop begins. Each insert during the benchmark will therefore
	// trigger the eviction path.
	for i := 0; i < 1000; i++ {
		n := &optimizedpkg.LiteralData{Value: i}
		c.Fetch(n, func() string { return "compiled" })
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := &optimizedpkg.LiteralData{Value: 1000 + i}
		c.Fetch(n, func() string { return "compiled" })
	}
}

// BenchmarkCacheFetchParallel measures cache Fetch performance when accessed
// concurrently. This can highlight the overhead introduced by the mutex
// guarding the cache map.
func BenchmarkCacheFetchParallel(b *testing.B) {
	c := optimizedpkg.NewCache()
	n := &optimizedpkg.LiteralData{Value: 42}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Fetch(n, func() string { return "compiled" })
		}
	})

}

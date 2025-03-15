package cypher

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Query represents a Cypher query under construction. It tracks
// registered parameters and accumulated clauses.
type Query struct {
	mu           sync.RWMutex
	parameters   map[string]interface{}
	paramCounter int
	clauses      []Clause
}

// NewQuery creates a new empty Query instance.
func NewQuery() *Query {
	return &Query{parameters: make(map[string]interface{})}
}

// RegisterParameter stores a value and returns its parameter key.
func (q *Query) RegisterParameter(value interface{}) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	for k, v := range q.parameters {
		if v == value {
			return k
		}
	}
	q.paramCounter++
	key := fmt.Sprintf("p%d", q.paramCounter)
	q.parameters[key] = value
	return key
}

// AddClause appends a clause to the query.
func (q *Query) AddClause(c Clause) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.clauses = append(q.clauses, c)
}

// BuildCypher assembles the full query string from its clauses.
func (q *Query) BuildCypher() (string, map[string]interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	sort.SliceStable(q.clauses, func(i, j int) bool {
		return ClauseOrder(q.clauses[i]) < ClauseOrder(q.clauses[j])
	})

	var b strings.Builder
	for i, c := range q.clauses {
		if i > 0 {
			b.WriteByte('\n') // Use newline for better readability between clauses
		}
		b.WriteString(c.BuildCypher(q))
	}
	return b.String(), q.parameters
}

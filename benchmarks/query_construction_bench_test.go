package benchmarks

import (
	"testing"

	"github.com/seuros/gopher-cypher/src/cypher"
)

func BenchmarkSimpleQueryConstruction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := cypher.NewQuery()
		q.AddClause(cypher.NewClauseAdapter(&cypher.MatchNode{Pattern: "(n)"}))
		q.AddClause(cypher.NewClauseAdapter(&cypher.ReturnNode{Items: []interface{}{"n"}}))
		q.BuildCypher()
	}
}

func BenchmarkComplexQueryConstruction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := cypher.NewQuery()
		q.AddClause(cypher.NewClauseAdapter(&cypher.MatchNode{Pattern: "(a)-[r]->(b)"}))

		cond1 := &cypher.ComparisonExpr{
			LHS: &cypher.PropertyAccessExpr{Variable: &cypher.LiteralExpr{Value: "a"}, PropertyName: "name"},
			Op:  "=",
			RHS: &cypher.LiteralExpr{Value: "foo"},
		}
		cond2 := &cypher.ComparisonExpr{
			LHS: &cypher.PropertyAccessExpr{Variable: &cypher.LiteralExpr{Value: "r"}, PropertyName: "since"},
			Op:  "<",
			RHS: &cypher.LiteralExpr{Value: 2020},
		}
		q.AddClause(cypher.NewClauseAdapter(&cypher.WhereNode{Conditions: []cypher.Expression{cond1, cond2}}))

		q.AddClause(cypher.NewClauseAdapter(&cypher.ReturnNode{Items: []interface{}{
			&cypher.PropertyAccessExpr{Variable: &cypher.LiteralExpr{Value: "a"}, PropertyName: "name"},
			&cypher.PropertyAccessExpr{Variable: &cypher.LiteralExpr{Value: "b"}, PropertyName: "name"},
			&cypher.PropertyAccessExpr{Variable: &cypher.LiteralExpr{Value: "r"}, PropertyName: "since"},
		}}))

		q.AddClause(cypher.NewClauseAdapter(&cypher.OrderByNode{Items: []cypher.OrderByItem{{
			Expression: &cypher.PropertyAccessExpr{Variable: &cypher.LiteralExpr{Value: "r"}, PropertyName: "since"},
			Direction:  "DESC",
		}}}))

		q.AddClause(cypher.NewClauseAdapter(&cypher.LimitNode{Expression: 10}))

		q.BuildCypher()
	}
}

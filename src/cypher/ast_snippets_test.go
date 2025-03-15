package cypher

import "testing"

// astRawExpr allows embedding a literal Cypher snippet as an Expression.
type astRawExpr string

// BuildCypher returns the raw Cypher string without parameterization.
func (r astRawExpr) BuildCypher(q *Query) string { return string(r) }

func compileNodesAST(nodes ...Node) (string, map[string]interface{}) {
	c := NewCompiler()
	c.Compile(nodes...)
	return c.Output(), c.parameters
}

func litNode(v interface{}) *LiteralNode { return &LiteralNode{Value: v} }

func TestASTSnippets(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []Node
		expected string
	}{
		{
			name:     "return_param",
			nodes:    []Node{&ReturnNode{Items: []interface{}{litNode(1)}}},
			expected: "RETURN $p1",
		},
		{
			name:     "return_two_params",
			nodes:    []Node{&ReturnNode{Items: []interface{}{litNode(1), litNode(2)}}},
			expected: "RETURN $p1, $p2",
		},
		{
			name:     "limit_param",
			nodes:    []Node{&LimitNode{Expression: litNode(10)}},
			expected: "LIMIT $p1",
		},
		{
			name:     "skip_param",
			nodes:    []Node{&SkipNode{Amount: litNode(5)}},
			expected: "SKIP $p1",
		},
		{
			name:     "where_eq",
			nodes:    []Node{&WhereNode{Conditions: []Expression{astRawExpr("$p1 = $p2")}}},
			expected: "WHERE $p1 = $p2",
		},
		{
			name:     "where_and",
			nodes:    []Node{&WhereNode{Conditions: []Expression{astRawExpr("($p1 > $p2 AND $p3 < $p4)")}}},
			expected: "WHERE ($p1 > $p2 AND $p3 < $p4)",
		},
		{
			name: "unwind_return",
			nodes: []Node{
				&UnwindNode{Expression: litNode([]int{1, 2}), AliasName: "x"},
				&ReturnNode{Items: []interface{}{"x"}},
			},
			expected: "UNWIND $p1 AS x\nRETURN x",
		},
		{
			name:     "return_aliases",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"$p1 AS name", "$p2 AS age"}}},
			expected: "RETURN $p1 AS name, $p2 AS age",
		},
		{
			name:     "return_collect",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"collect($p1) AS items"}}},
			expected: "RETURN collect($p1) AS items",
		},
		{
			name:     "return_size_cmp",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"size($p1) > $p2"}}},
			expected: "RETURN size($p1) > $p2",
		},
		{
			name:     "return_math",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"$p1 + $p2", "$p3 - $p4"}}},
			expected: "RETURN $p1 + $p2, $p3 - $p4",
		},
		{
			name:     "return_coalesce",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"coalesce($p1, $p2, $p3)"}}},
			expected: "RETURN coalesce($p1, $p2, $p3)",
		},
		{
			name:     "return_case",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"CASE WHEN $p1 = $p2 THEN $p3 ELSE $p4 END"}}},
			expected: "RETURN CASE WHEN $p1 = $p2 THEN $p3 ELSE $p4 END",
		},
		{
			name: "with_where_return",
			nodes: []Node{
				&WithNode{Items: []interface{}{"$p1", "$p2"}, Distinct: true, WhereConditions: []interface{}{"$p3 > $p4"}},
				&ReturnNode{Items: []interface{}{"$p5"}},
			},
			expected: "WITH DISTINCT $p1, $p2\nWHERE $p3 > $p4\nRETURN $p5",
		},
		{
			name:     "set_property",
			nodes:    []Node{&SetNode{Assignments: []SetAssignment{PropertyAssignment{"n.name", "$p1"}}}},
			expected: "SET n.name = $p1",
		},
		{
			name:     "set_merge",
			nodes:    []Node{&SetNode{Assignments: []SetAssignment{VariablePropertiesAssignment{"n", "$p1", true}}}},
			expected: "SET n += $p1",
		},
		{
			name:     "remove_property",
			nodes:    []Node{&RemoveNode{Items: []RemoveItem{PropertyRemoval{"n.age"}}}},
			expected: "REMOVE n.age",
		},
		{
			name:     "remove_label",
			nodes:    []Node{&RemoveNode{Items: []RemoveItem{LabelRemoval{"n", "Old"}}}},
			expected: "REMOVE n:Old",
		},
		{
			name:     "delete_param",
			nodes:    []Node{&DeleteNode{Expressions: []interface{}{"$p1"}}},
			expected: "DELETE $p1",
		},
		{
			name:     "detach_delete_param",
			nodes:    []Node{&DeleteNode{Expressions: []interface{}{"$p1"}, Detach: true}},
			expected: "DETACH DELETE $p1",
		},
		{
			name: "foreach_set",
			nodes: []Node{
				&ForeachNode{Variable: "x", Expression: "$p1", UpdateClauses: []Node{
					&SetNode{Assignments: []SetAssignment{PropertyAssignment{"x.name", "$p2"}}},
				}},
			},
			expected: "FOREACH (x IN $p1 | SET x.name = $p2)",
		},
		{
			name: "call_labels",
			nodes: []Node{
				&ProcedureCallNode{Procedure: "db.labels()", YieldItems: []string{"label"}},
				&ReturnNode{Items: []interface{}{"label"}},
			},
			expected: "CALL db.labels() YIELD label\nRETURN label",
		},
		{
			name: "call_subquery_param",
			nodes: []Node{
				&CallSubqueryNode{Body: []Node{&ReturnNode{Items: []interface{}{"$p1 AS val"}}}},
				&ReturnNode{Items: []interface{}{"val"}},
			},
			expected: "CALL { RETURN $p1 AS val }\nRETURN val",
		},
		{
			name: "match_param",
			nodes: []Node{
				&MatchNode{Pattern: "(n:User {id: $p1})"},
				&ReturnNode{Items: []interface{}{"n"}},
			},
			expected: "MATCH (n:User {id: $p1})\nRETURN n",
		},
		{
			name: "merge_param",
			nodes: []Node{
				&MergeNode{Pattern: "(n:User {email: $p1})", OnCreate: &SetNode{Assignments: []SetAssignment{PropertyAssignment{"n.created_at", "$p2"}}}},
			},
			expected: "MERGE (n:User {email: $p1}) ON CREATE SET n.created_at = $p2",
		},
		{
			name: "loadcsv_param",
			nodes: []Node{
				&LoadCSVNode{WithHeaders: true, From: "$p1", As: "row"},
				&ReturnNode{Items: []interface{}{"row"}},
			},
			expected: "LOAD CSV WITH HEADERS FROM $p1 AS row\nRETURN row",
		},
		{
			name:     "return_const_int",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"1"}}},
			expected: "RETURN 1",
		},
		{
			name:     "return_const_string",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"\"hello\""}}},
			expected: "RETURN \"hello\"",
		},
		{
			name:     "return_const_mixed",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"1", "\"world\""}}},
			expected: "RETURN 1, \"world\"",
		},
		{
			name:     "limit_const",
			nodes:    []Node{&LimitNode{Expression: "10"}},
			expected: "LIMIT 10",
		},
		{
			name:     "skip_const",
			nodes:    []Node{&SkipNode{Amount: "5"}},
			expected: "SKIP 5",
		},
		{
			name:     "where_const_eq",
			nodes:    []Node{&WhereNode{Conditions: []Expression{astRawExpr("10 = 10")}}},
			expected: "WHERE 10 = 10",
		},
		{
			name:     "where_const_and",
			nodes:    []Node{&WhereNode{Conditions: []Expression{astRawExpr("(42 > 21 AND 3 < 7)")}}},
			expected: "WHERE (42 > 21 AND 3 < 7)",
		},
		{
			name: "unwind_const",
			nodes: []Node{
				&UnwindNode{Expression: "[1,2,3]", AliasName: "x"},
				&ReturnNode{Items: []interface{}{"x"}},
			},
			expected: "UNWIND [1,2,3] AS x\nRETURN x",
		},
		{
			name:     "return_const_aliases",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"\"foo\" AS name", "123 AS age"}}},
			expected: "RETURN \"foo\" AS name, 123 AS age",
		},
		{
			name:     "return_const_collect",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"collect(\"a\") AS items"}}},
			expected: "RETURN collect(\"a\") AS items",
		},
		{
			name:     "return_const_size_cmp",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"size([1,2,3]) > 0"}}},
			expected: "RETURN size([1,2,3]) > 0",
		},
		{
			name:     "return_const_math",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"5 + 3", "9 - 1"}}},
			expected: "RETURN 5 + 3, 9 - 1",
		},
		{
			name:     "return_const_coalesce",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"coalesce(\"a\", \"b\", \"c\")"}}},
			expected: "RETURN coalesce(\"a\", \"b\", \"c\")",
		},
		{
			name:     "return_const_case",
			nodes:    []Node{&ReturnNode{Items: []interface{}{"CASE WHEN 1 = 1 THEN \"ok\" ELSE \"fail\" END"}}},
			expected: "RETURN CASE WHEN 1 = 1 THEN \"ok\" ELSE \"fail\" END",
		},
		{
			name: "with_const_where_return",
			nodes: []Node{
				&WithNode{Items: []interface{}{"\"a\"", "\"b\""}, Distinct: true, WhereConditions: []interface{}{"1 > 0"}},
				&ReturnNode{Items: []interface{}{"1"}},
			},
			expected: "WITH DISTINCT \"a\", \"b\"\nWHERE 1 > 0\nRETURN 1",
		},
		{
			name:     "set_const_property",
			nodes:    []Node{&SetNode{Assignments: []SetAssignment{PropertyAssignment{"n.name", "\"CHAD\""}}}},
			expected: "SET n.name = \"CHAD\"",
		},
		{
			name:     "set_const_merge",
			nodes:    []Node{&SetNode{Assignments: []SetAssignment{VariablePropertiesAssignment{"n", "{age: 30}", true}}}},
			expected: "SET n += {age: 30}",
		},
		{
			name:     "remove_const_property",
			nodes:    []Node{&RemoveNode{Items: []RemoveItem{PropertyRemoval{"n.age"}}}},
			expected: "REMOVE n.age",
		},
		{
			name:     "remove_const_label",
			nodes:    []Node{&RemoveNode{Items: []RemoveItem{LabelRemoval{"n", "Deprecated"}}}},
			expected: "REMOVE n:Deprecated",
		},
		{
			name:     "delete_const",
			nodes:    []Node{&DeleteNode{Expressions: []interface{}{"n"}}},
			expected: "DELETE n",
		},
		{
			name:     "detach_delete_const",
			nodes:    []Node{&DeleteNode{Expressions: []interface{}{"n"}, Detach: true}},
			expected: "DETACH DELETE n",
		},
		{
			name: "foreach_const",
			nodes: []Node{
				&ForeachNode{Variable: "x", Expression: "[\"a\", \"b\"]", UpdateClauses: []Node{
					&SetNode{Assignments: []SetAssignment{PropertyAssignment{"x.foo", "\"bar\""}}},
				}},
			},
			expected: "FOREACH (x IN [\"a\", \"b\"] | SET x.foo = \"bar\")",
		},
		{
			name: "call_labels_const",
			nodes: []Node{
				&ProcedureCallNode{Procedure: "db.labels()", YieldItems: []string{"label"}},
				&ReturnNode{Items: []interface{}{"label"}},
			},
			expected: "CALL db.labels() YIELD label\nRETURN label",
		},
		{
			name: "call_subquery_const",
			nodes: []Node{
				&CallSubqueryNode{Body: []Node{&ReturnNode{Items: []interface{}{"42 AS val"}}}},
				&ReturnNode{Items: []interface{}{"val"}},
			},
			expected: "CALL { RETURN 42 AS val }\nRETURN val",
		},
		{
			name: "match_const",
			nodes: []Node{
				&MatchNode{Pattern: "(n:User {id: 99})"},
				&ReturnNode{Items: []interface{}{"n"}},
			},
			expected: "MATCH (n:User {id: 99})\nRETURN n",
		},
		{
			name: "merge_const",
			nodes: []Node{
				&MergeNode{Pattern: "(n:User {email: \"test@example.com\"})", OnCreate: &SetNode{Assignments: []SetAssignment{PropertyAssignment{"n.created_at", "timestamp()"}}}},
			},
			expected: "MERGE (n:User {email: \"test@example.com\"}) ON CREATE SET n.created_at = timestamp()",
		},
		{
			name: "loadcsv_const",
			nodes: []Node{
				&LoadCSVNode{WithHeaders: true, From: "\"file:///data.csv\"", As: "row"},
				&ReturnNode{Items: []interface{}{"row"}},
			},
			expected: "LOAD CSV WITH HEADERS FROM \"file:///data.csv\" AS row\nRETURN row",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := compileNodesAST(tc.nodes...)
			if out != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, out)
			}
		})
	}
}

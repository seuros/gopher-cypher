# Parser and AST Examples

This document provides examples of how to work with the Cypher Abstract Syntax Tree (AST) in this project.

## AST Node Structure

The foundation of the AST is the `Node` interface, defined in `src/cypher/node.go`. It's a simple interface:

```go
package cypher

// Node represents a single AST element. It participates in the visitor
// pattern used by the compilers.
type Node interface {
	// Accept allows a visitor to process the node.
	Accept(v Visitor) error
}

// Visitor is implemented by types that can handle specific AST nodes.
type Visitor interface{}
```

Various structs within the `src/cypher/` directory implement this `Node` interface. Each of these structs represents a different component or clause of a Cypher query. For example, `LiteralNode` might represent a literal value (like a string or number), `ReturnNode` could represent a `RETURN` clause, and `MatchNode` could represent a `MATCH` clause.

These nodes are then typically processed by a `Compiler` (found in `src/cypher/compiler.go`), which traverses the AST (often using the `Accept` method and a `Visitor` pattern) to generate the final Cypher query string.

## Simple AST Example: RETURN a Literal

Let's create a very simple query: `RETURN "Hello, AST!"`.

This involves two main AST nodes:
1.  A `LiteralNode` to represent the string `"Hello, AST!"`.
2.  A `ReturnNode` to represent the `RETURN` clause, containing the `LiteralNode`.

Here's how you might construct and compile this:

```go
package main

import (
		"fmt"

		"github.com/seuros/gopher-cypher/src/cypher"
	)

func main() {
	// 1. Create a LiteralNode for the string
	literal := &cypher.LiteralNode{Value: "Hello, AST!"}

	// 2. Create a ReturnNode
	// The Items field expects a slice of interfaces.
	// If ReturnNode expects specific types or Expression, this might need adjustment.
	// Based on compiler.go, renderExpression handles various types, including LiteralNode.
	returnClause := &cypher.ReturnNode{
		Items: []interface{}{literal},
	}

	// 3. Compile the AST
	compiler := cypher.NewCompiler()
	// The Compile method takes a variadic Node argument
	queryString, params := compiler.Compile(returnClause)

	fmt.Println("Generated Cypher Query:")
	fmt.Println(queryString)
	fmt.Println("Parameters:")
	for k, v := range params {
		fmt.Printf("%s: %v\n", k, v)
	}

	// Expected Output:
	// Generated Cypher Query:
	// RETURN $p1
	// Parameters:
	// p1: Hello, AST!
}

```

**Note:** The exact structure of `ReturnNode.Items` and how expressions or nodes are added might need verification against the actual `ReturnNode` struct definition in `src/cypher/return_node.go` (or wherever it's defined). The example above assumes `Items` can take `Node` instances directly or general `interface{}` that the compiler's `renderExpression` can handle. The parameter name `$p1` might vary depending on the compiler's internal state.

## Complex AST Example: MATCH, WHERE, RETURN

Let's construct a query like: `MATCH (n:Person) WHERE n.name = "CHAD" RETURN n.age`

This involves:
1.  A `MatchNode` with a simple pattern string.
2.  A `WhereNode` holding a comparison expression.
3.  A `ReturnNode` selecting a property.

Because variable identifiers should not be parameterized, this example uses a small `RawExpr` helper to inline them.

```go
package main

import (
	"fmt"

	"github.com/seuros/gopher-cypher/src/cypher"
)

// RawExpr is a small helper for inlining identifiers or snippets that should not be parameterized.
type RawExpr string

func (r RawExpr) BuildCypher(q *cypher.Query) string {
	return string(r)
}

func main() {
	matchClause := &cypher.MatchNode{
		Pattern: "(n:Person)",
	}

	whereClause := &cypher.WhereNode{
		Conditions: []cypher.Expression{
			&cypher.ComparisonExpr{
				LHS: RawExpr("n.name"),
				Op:  "=",
				RHS: &cypher.LiteralExpr{Value: "CHAD"},
			},
		},
	}

	returnClause := &cypher.ReturnNode{
		Items: []interface{}{RawExpr("n.age")},
	}

	compiler := cypher.NewCompiler()
	queryString, params := compiler.Compile(matchClause, whereClause, returnClause)

	fmt.Println(queryString)
	fmt.Println("Parameters:", params)
}
```

**Important Considerations:**

*   **Pattern Representation:** The `PatternNode` and its construction are speculative. You'll need to investigate `src/cypher/pattern_node.go` (if it exists) or see how patterns are built within `MatchNode` or `MergeNode`.
*   **Expression System:** Cypher queries involve complex expressions (e.g., `n.name`, `n.age`, comparisons). There's likely a dedicated system or set of structs for representing these (e.g., `PropertyExpression`, `ComparisonExpression`, `FunctionCallExpression`). The `Expression` struct used above is a placeholder.
*   **Parameterization:** The compiler automatically handles parameterization for `LiteralNode`s. For more complex expressions, ensure they are constructed in a way that allows the compiler to identify and parameterize values correctly. The example above simplifies this aspect.

You would need to consult the specific struct definitions in `src/cypher/` (e.g., `match_node.go`, `where_node.go`, `expression.go`) to build this example accurately.

## Role of the `Query` Object vs. `Compiler`

In `src/cypher/query.go`, there's a `Query` struct that provides methods like `AddClause()` and `BuildCypher()`. This object appears to be a higher-level builder for constructing Cypher queries by accumulating "clauses."

While both `Query` and `Compiler` deal with generating Cypher strings and managing parameters, their roles in the context of the AST (as represented by `Node` implementations) seem distinct:

*   **`Compiler` (`src/cypher/compiler.go`):** This is the primary tool for transforming a tree of AST `Node` objects into a Cypher query string. You manually construct the AST nodes (like `LiteralNode`, `ReturnNode`, `MatchNode`, etc.) and then pass them to `compiler.Compile()`. The compiler traverses these nodes (using a Visitor pattern) to generate the query.

*   **`Query` (`src/cypher/query.go`):** This object seems to offer a more programmatic way to build queries by adding pre-defined `Clause` objects. It's not explicitly designed to take a raw, user-constructed AST made of `Node` interfaces in the same way the `Compiler` does. The `Clause` interface used by `Query.AddClause()` might be a different abstraction than the `Node` interface.

**Conclusion for AST Manipulation:**

If you are working directly with AST `Node` objects (i.e., manually building or manipulating the tree structure of a query), the **`Compiler`** is the more relevant component to use for generating the final Cypher query string.

The `Query` object might be useful for other query construction scenarios, but the examples in this document focus on the direct creation and compilation of ASTs using the `Node` implementations and the `Compiler`.

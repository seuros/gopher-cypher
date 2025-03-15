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
	"log"

	"github.com/seuros/gopher-cypher/src/cypher" // Assuming this is the correct import path
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
		fmt.Printf("%s: %v
", k, v)
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
1.  A `PatternNode` for `(n:Person)`. (Assuming a `PatternNode` or similar construct exists. The actual representation might involve `NodeIdentifier`, `Label` etc. within a `MatchNode`'s pattern field).
2.  A `MatchNode` to encapsulate the pattern.
3.  An `Expression` or a specific node type for the `n.name = "CHAD"` condition. This might be a `ComparisonExpression` or similar.
4.  A `WhereNode` to hold the condition.
5.  An `Expression` for `n.age`.
6.  A `ReturnNode` for `RETURN n.age`.

```go
package main

import (
	"fmt"
	"log"

	"github.com/seuros/gopher-cypher/src/cypher" // Assuming this is the correct import path
)

func main() {
	// It's highly likely that specific structs exist for patterns, expressions, etc.
	// This is a conceptual example. The actual node construction will depend
	// on the precise definitions in the src/cypher/ package.

	// Representing the pattern (n:Person)
	// This is speculative. The actual API for creating patterns might be different.
	// For instance, there might be functions like cypher.NodePattern("n", "Person", nil)
	pattern := cypher.PatternNode{ // Assuming PatternNode exists
		Path: []cypher.PathElement{ // PathElement is also an assumption
			{
				Node: &cypher.NodePattern{ // NodePattern is an assumption
					Variable: "n",
					Labels:   []string{"Person"},
				},
			},
		},
	}

	// 1. MatchNode
	matchClause := &cypher.MatchNode{
		Pattern: pattern, // Or however patterns are supplied
	}

	// 2. WhereNode
	// Representing n.name = "CHAD"
	// This likely involves an Expression type.
	// Example: cypher.Equals(cypher.Property("n", "name"), cypher.Literal("CHAD"))
	// For simplicity, we'll use a placeholder string, though in reality,
	// this would be a structured Expression.
	whereCondition := cypher.Expression{ // Placeholder, this would be a structured expression
		// This is a simplified representation.
		// Actual implementation would use specific expression types.
		Representation: "n.name = $param1", // Example, actual structure needed
	}
	// The compiler handles parameter registration. Let's assume "CHAD" would be registered.
	// To make this runnable, we'd need to ensure "CHAD" is passed in a way the
	// compiler can create a parameter for it, or use a LiteralNode if appropriate.
	// For the purpose of this example, let's assume the Expression handles it.

	whereClause := &cypher.WhereNode{
		Conditions: []cypher.Expression{whereCondition},
	}

	// 3. ReturnNode
	// Representing n.age
	// Example: cypher.Property("n", "age")
	returnItem := cypher.Expression{ // Placeholder
		Representation: "n.age",
	}
	returnClause := &cypher.ReturnNode{
		Items: []interface{}{returnItem},
	}

	// 4. Compile the AST
	compiler := cypher.NewCompiler()
	queryString, params := compiler.Compile(matchClause, whereClause, returnClause) // Pass nodes in order

	fmt.Println("Generated Cypher Query:")
	fmt.Println(queryString)
	fmt.Println("Parameters:")
	// In a real scenario, "CHAD" would be in params if `whereCondition` was a proper LiteralNode
	// or if the Expression system registered it.
	// For this conceptual example, params might be empty or contain what the
	// simplified Expression placeholder implied. Let's assume "CHAD" became p1.
	// To make this fully work, `literalCHAD := &cypher.LiteralNode{Value: "CHAD"}`
	// would be used in the expression for `n.name = $p1`
	// and params would then contain `p1: "CHAD"`.

	// A more realistic parameter setup if LiteralNode was used for "CHAD":
	// params["param1"] = "CHAD" // Or whatever key the compiler generates

	fmt.Println("--- Conceptual Output ---")
	fmt.Println("MATCH (n:Person)")
	fmt.Println("WHERE n.name = $p1") // Assuming "CHAD" is parameterized
	fmt.Println("RETURN n.age")
	fmt.Println("Parameters: {p1: "CHAD"}") // Ideal parameters

	// Actual output will depend heavily on how PatternNode and Expression are truly implemented
	// and how they interact with the compiler for parameterization.
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

package cypher

import "fmt"

// Expression defines any value that can appear in a Cypher statement.
type Expression interface {
	// BuildCypher returns the Cypher representation of the expression
	// while appending parameters to the Query.
	BuildCypher(q *Query) string
}

// ComparisonExpr represents a comparison expression (e.g., a = b).
type ComparisonExpr struct {
	LHS Expression
	RHS Expression
	Op  string
}

// BuildCypher implements the Expression interface for ComparisonExpr.
func (e *ComparisonExpr) BuildCypher(q *Query) string {
	return e.LHS.BuildCypher(q) + " " + e.Op + " " + e.RHS.BuildCypher(q)
}

// PropertyAccessExpr represents accessing a property on a variable (e.g., n.name).
type PropertyAccessExpr struct {
	Variable     Expression
	PropertyName string
}

// BuildCypher implements the Expression interface for PropertyAccessExpr.
func (e *PropertyAccessExpr) BuildCypher(q *Query) string {
	return e.Variable.BuildCypher(q) + "." + e.PropertyName
}

// LiteralExpr represents a literal value (e.g., "string", 123, true).
type LiteralExpr struct {
	Value interface{}
}

// BuildCypher implements the Expression interface for LiteralExpr.
func (e *LiteralExpr) BuildCypher(q *Query) string {
	paramKey := q.RegisterParameter(e.Value)
	return fmt.Sprintf("$%s", paramKey)
}

// FunctionCallExpr represents a function call (e.g., collect(n), coalesce(a, b)).
type FunctionCallExpr struct {
	Name      string
	Arguments []interface{}
}

// BuildCypher implements the Expression interface for FunctionCallExpr.
func (e *FunctionCallExpr) BuildCypher(q *Query) string {
	result := e.Name + "("
	for i, arg := range e.Arguments {
		if i > 0 {
			result += ", "
		}
		paramKey := q.RegisterParameter(arg)
		result += fmt.Sprintf("$%s", paramKey)
	}
	result += ")"
	return result
}

// AliasExpr represents an expression with an alias (e.g., expr AS alias).
type AliasExpr struct {
	Expression interface{}
	Alias      string
}

// BuildCypher implements the Expression interface for AliasExpr.
func (e *AliasExpr) BuildCypher(q *Query) string {
	var exprStr string
	if expr, ok := e.Expression.(Expression); ok {
		exprStr = expr.BuildCypher(q)
	} else {
		paramKey := q.RegisterParameter(e.Expression)
		exprStr = fmt.Sprintf("$%s", paramKey)
	}
	return exprStr + " AS " + e.Alias
}

// MathExpr represents a mathematical expression (e.g., a + b, x - y).
type MathExpr struct {
	Left     interface{}
	Operator string
	Right    interface{}
}

// BuildCypher implements the Expression interface for MathExpr.
func (e *MathExpr) BuildCypher(q *Query) string {
	leftStr := ""
	rightStr := ""

	if leftParam := q.RegisterParameter(e.Left); leftParam != "" {
		leftStr = "$" + leftParam
	}
	if rightParam := q.RegisterParameter(e.Right); rightParam != "" {
		rightStr = "$" + rightParam
	}

	return leftStr + " " + e.Operator + " " + rightStr
}

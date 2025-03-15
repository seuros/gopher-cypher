package cypher

import (
	"fmt"
	"strings"

	"github.com/seuros/gopher-cypher/src/optimized"
)

// Compiler walks the AST and builds Cypher output.
type Compiler struct {
	output       strings.Builder
	parameters   map[string]interface{}
	paramCounter int
	firstClause  bool
}

// NewCompiler creates a new compiler instance.
func NewCompiler() *Compiler {
	return &Compiler{parameters: make(map[string]interface{}), firstClause: true}
}

// Output returns the compiled query string.
func (c *Compiler) Output() string { return c.output.String() }

// Compile compiles one or more AST nodes.
func (c *Compiler) Compile(nodes ...Node) (string, map[string]interface{}) {
	for _, n := range nodes {
		if !c.firstClause {
			c.output.WriteByte('\n')
		}
		n.Accept(c)
		c.firstClause = false
	}
	return c.output.String(), c.parameters
}

// internal helper to register parameters
func (c *Compiler) registerParameter(val interface{}) string {
	for k, v := range c.parameters {
		if v == val {
			return k
		}
	}
	c.paramCounter++
	key := fmt.Sprintf("p%d", c.paramCounter)
	c.parameters[key] = val
	return key
}

// VisitLiteralNode renders a literal value.
func (c *Compiler) VisitLiteralNode(n *LiteralNode) error {
	key := c.registerParameter(n.Value)
	c.output.WriteString("$" + key)
	return nil
}

// VisitLiteralData renders an optimized literal value.
func (c *Compiler) VisitLiteralData(n *LiteralData) error {
	return c.VisitLiteralNode(&LiteralNode{Value: n.Value})
}

// VisitOptimizedLiteralData handles optimized package literals.
func (c *Compiler) VisitOptimizedLiteralData(n *optimized.LiteralData) error {
	return c.VisitLiteralNode(&LiteralNode{Value: n.Value})
}

// helper to render expressions or raw values
func (c *Compiler) renderExpression(expr interface{}) {
	switch v := expr.(type) {
	case Expression:
		// Create a temporary Query facade for the Expression to use.
		// This allows Expression.BuildCypher to call RegisterParameter,
		// which might be overridden by QueryIntegratedCompiler to use its own Query instance.
		tempQuery := &Query{parameters: c.parameters, paramCounter: c.paramCounter}
		c.output.WriteString(v.BuildCypher(tempQuery))
		// Update the compiler's paramCounter if the Expression registered new params.
		c.paramCounter = tempQuery.paramCounter
	case Node:
		v.Accept(c)
	case string:
		c.output.WriteString(v)
	case []interface{}:
		c.output.WriteString(c.formatArrayLiteral(v))
	default:
		c.VisitLiteralNode(&LiteralNode{Value: v})
	}
}

func (c *Compiler) formatArrayLiteral(arr []interface{}) string {
	parts := make([]string, len(arr))
	for i, el := range arr {
		switch v := el.(type) {
		case string:
			parts[i] = fmt.Sprintf("'%s'", v)
		case []interface{}:
			parts[i] = c.formatArrayLiteral(v)
		default:
			parts[i] = fmt.Sprint(v)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// VisitSetNode handles SET clauses
func (c *Compiler) VisitSetNode(n *SetNode) error {
	if len(n.Assignments) == 0 {
		return nil
	}
	c.output.WriteString("SET ")
	for i, asn := range n.Assignments {
		if i > 0 {
			c.output.WriteString(", ")
		}
		c.renderAssignment(asn)
	}
	return nil
}

func (c *Compiler) renderAssignment(a SetAssignment) {
	switch v := a.(type) {
	case PropertyAssignment:
		c.output.WriteString(v.Property)
		c.output.WriteString(" = ")
		c.renderExpression(v.Value)
	case VariablePropertiesAssignment:
		c.output.WriteString(v.Variable)
		if v.Merge {
			c.output.WriteString(" += ")
		} else {
			c.output.WriteString(" = ")
		}
		c.renderExpression(v.Value)
	case LabelAssignment:
		c.output.WriteString(fmt.Sprintf("%s:%s", v.Variable, v.Label))
	default:
		c.output.WriteString(fmt.Sprint(v))
	}
}

// VisitRemoveNode handles REMOVE clauses
func (c *Compiler) VisitRemoveNode(n *RemoveNode) error {
	c.output.WriteString("REMOVE ")
	for i, item := range n.Items {
		if i > 0 {
			c.output.WriteString(", ")
		}
		switch v := item.(type) {
		case PropertyRemoval:
			c.output.WriteString(v.Property)
		case LabelRemoval:
			c.output.WriteString(fmt.Sprintf("%s:%s", v.Variable, v.Label))
		case string:
			c.output.WriteString(v)
		default:
			c.output.WriteString(fmt.Sprint(v))
		}
	}
	return nil
}

// VisitReturnNode handles RETURN clauses
func (c *Compiler) VisitReturnNode(n *ReturnNode) error {
	c.output.WriteString("RETURN ")
	if n.Distinct {
		c.output.WriteString("DISTINCT ")
	}
	for i, item := range n.Items {
		if i > 0 {
			c.output.WriteString(", ")
		}
		c.renderExpression(item)
	}
	return nil
}

// VisitWithNode handles WITH clauses
func (c *Compiler) VisitWithNode(n *WithNode) error {
	c.output.WriteString("WITH ")
	if n.Distinct {
		c.output.WriteString("DISTINCT ")
	}
	for i, item := range n.Items {
		if i > 0 {
			c.output.WriteString(", ")
		}
		c.renderExpression(item)
	}
	if len(n.WhereConditions) > 0 {
		c.output.WriteString("\nWHERE ")
		for i, cond := range n.WhereConditions {
			if i > 0 {
				c.output.WriteString(" AND ")
			}
			c.renderExpression(cond)
		}
	}
	return nil
}

// VisitUnwindNode handles UNWIND clauses
func (c *Compiler) VisitUnwindNode(n *UnwindNode) error {
	c.output.WriteString("UNWIND ")
	switch v := n.Expression.(type) {
	case []interface{}:
		c.output.WriteString(c.formatArrayLiteral(v))
	default:
		c.renderExpression(v)
	}
	c.output.WriteString(" AS ")
	c.output.WriteString(n.AliasName)
	return nil
}

// VisitForeachNode handles FOREACH clauses
func (c *Compiler) VisitForeachNode(n *ForeachNode) error {
	c.output.WriteString("FOREACH (")
	c.output.WriteString(n.Variable)
	c.output.WriteString(" IN ")
	c.renderExpression(n.Expression)
	c.output.WriteString(" | ")
	for i, cl := range n.UpdateClauses {
		if i > 0 {
			c.output.WriteByte(' ')
		}
		cl.Accept(c)
	}
	c.output.WriteString(")")
	return nil
}

// VisitWhereNode handles WHERE clauses
func (c *Compiler) VisitWhereNode(n *WhereNode) error {
	if len(n.Conditions) == 0 {
		return nil
	}
	c.output.WriteString("WHERE ")
	for i, cond := range n.Conditions {
		if i > 0 {
			c.output.WriteString(" AND ")
		}
		c.renderExpression(cond)
	}
	return nil
}

// VisitSkipNode handles SKIP clauses
func (c *Compiler) VisitSkipNode(n *SkipNode) error {
	c.output.WriteString("SKIP ")
	c.renderExpression(n.Amount)
	return nil
}

// VisitLimitNode handles LIMIT clauses
func (c *Compiler) VisitLimitNode(n *LimitNode) error {
	c.output.WriteString("LIMIT ")
	c.renderExpression(n.Expression)
	return nil
}

// VisitSkipData handles optimized SkipData nodes.
func (c *Compiler) VisitSkipData(n *optimized.SkipData) error {
	return c.VisitSkipNode(&SkipNode{Amount: n.Expression})
}

// VisitLimitData handles optimized LimitData nodes.
func (c *Compiler) VisitLimitData(n *optimized.LimitData) error {
	return c.VisitLimitNode(&LimitNode{Expression: n.Expression})
}

// VisitOrderByNode handles ORDER BY clauses
func (c *Compiler) VisitOrderByNode(n *OrderByNode) error {
	c.output.WriteString("ORDER BY ")
	for i, item := range n.Items {
		if i > 0 {
			c.output.WriteString(", ")
		}
		c.renderExpression(item.Expression)
		dir := strings.ToUpper(item.Direction)
		if dir != "" && dir != "ASC" {
			c.output.WriteByte(' ')
			c.output.WriteString(dir)
		}
	}
	return nil
}

// VisitMatchNode handles MATCH clauses
func (c *Compiler) VisitMatchNode(n *MatchNode) error {
	c.output.WriteString("MATCH ")
	c.renderExpression(n.Pattern)
	return nil
}

// VisitMergeNode handles MERGE clauses
func (c *Compiler) VisitMergeNode(n *MergeNode) error {
	c.output.WriteString("MERGE ")
	c.renderExpression(n.Pattern)
	if n.OnCreate != nil {
		c.output.WriteString(" ON CREATE ")
		n.OnCreate.Accept(c)
	}
	return nil
}

// VisitProcedureCallNode handles CALL procedure clauses
func (c *Compiler) VisitProcedureCallNode(n *ProcedureCallNode) error {
	c.output.WriteString("CALL ")
	c.renderExpression(n.Procedure)
	if len(n.YieldItems) > 0 {
		c.output.WriteString(" YIELD ")
		for i, y := range n.YieldItems {
			if i > 0 {
				c.output.WriteString(", ")
			}
			c.output.WriteString(y)
		}
	}
	return nil
}

// VisitCallSubqueryNode handles CALL { ... } subqueries
func (c *Compiler) VisitCallSubqueryNode(n *CallSubqueryNode) error {
	c.output.WriteString("CALL {")
	origFirst := c.firstClause
	c.firstClause = true
	for i, node := range n.Body {
		if i > 0 {
			c.output.WriteByte('\n')
		} else {
			c.output.WriteByte(' ')
		}
		node.Accept(c)
		c.firstClause = false
	}
	c.output.WriteString(" }")
	c.firstClause = origFirst
	return nil
}

// VisitDeleteNode handles DELETE clauses
func (c *Compiler) VisitDeleteNode(n *DeleteNode) error {
	if n.Detach {
		c.output.WriteString("DETACH DELETE ")
	} else {
		c.output.WriteString("DELETE ")
	}
	for i, expr := range n.Expressions {
		if i > 0 {
			c.output.WriteString(", ")
		}
		c.renderExpression(expr)
	}
	return nil
}

// VisitLoadCSVNode handles LOAD CSV clauses
func (c *Compiler) VisitLoadCSVNode(n *LoadCSVNode) error {
	c.output.WriteString("LOAD CSV ")
	if n.WithHeaders {
		c.output.WriteString("WITH HEADERS ")
	}
	c.output.WriteString("FROM ")
	c.renderExpression(n.From)
	if n.As != "" {
		c.output.WriteString(" AS ")
		c.output.WriteString(n.As)
	}
	return nil
}

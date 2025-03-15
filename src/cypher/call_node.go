package cypher

// ProcedureCallNode represents a CALL of a procedure with optional YIELD items.
type ProcedureCallNode struct {
	Procedure  interface{}
	YieldItems []string
}

func (n *ProcedureCallNode) Accept(v Visitor) error {
	if vv, ok := v.(interface {
		VisitProcedureCallNode(*ProcedureCallNode) error
	}); ok {
		return vv.VisitProcedureCallNode(n)
	}
	return nil
}

// CallSubqueryNode represents CALL { ... } constructs.
type CallSubqueryNode struct {
	Body []Node
}

func (n *CallSubqueryNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitCallSubqueryNode(*CallSubqueryNode) error }); ok {
		return vv.VisitCallSubqueryNode(n)
	}
	return nil
}

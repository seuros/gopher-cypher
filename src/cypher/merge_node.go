package cypher

// MergeNode represents a MERGE clause with optional ON CREATE actions.
type MergeNode struct {
	Pattern  interface{}
	OnCreate Node // optional clause executed on CREATE
}

func (n *MergeNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitMergeNode(*MergeNode) error }); ok {
		return vv.VisitMergeNode(n)
	}
	return nil
}

// Type returns the ClauseType for MergeNode.
func (n *MergeNode) Type() ClauseType {
	return MergeClause
}

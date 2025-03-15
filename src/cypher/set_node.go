package cypher

// SetAssignment represents a single SET operation.
type SetAssignment interface{}

type PropertyAssignment struct {
	Property string
	Value    interface{}
}

type VariablePropertiesAssignment struct {
	Variable string
	Value    interface{}
	Merge    bool
}

type LabelAssignment struct {
	Variable string
	Label    string
}

// SetNode represents a SET clause.
type SetNode struct {
	Assignments []SetAssignment
}

func (n *SetNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitSetNode(*SetNode) error }); ok {
		return vv.VisitSetNode(n)
	}
	return nil
}

// Type returns the ClauseType for SetNode.
func (n *SetNode) Type() ClauseType {
	return SetClause
}

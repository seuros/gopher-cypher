package optimized

// Node represents a minimal AST element used by the optimized package.
type Node interface {
	// Accept allows a visitor to process the node.
	Accept(v Visitor) error
}

// Visitor defines visit methods that optimized nodes can call.
type Visitor interface{}

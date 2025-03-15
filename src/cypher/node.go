package cypher

// Node represents a single AST element. It participates in the visitor
// pattern used by the compilers.
type Node interface {
	// Accept allows a visitor to process the node.
	Accept(v Visitor) error
}

// Visitor is implemented by types that can handle specific AST nodes.
type Visitor interface{}

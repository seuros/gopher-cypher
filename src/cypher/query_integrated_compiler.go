package cypher

// QueryIntegratedCompiler is a Compiler that registers parameters into
// an external Query instance.
type QueryIntegratedCompiler struct {
	*Compiler
	query *Query
}

// NewQueryIntegratedCompiler creates a compiler bound to a Query.
func NewQueryIntegratedCompiler(q *Query) *QueryIntegratedCompiler {
	return &QueryIntegratedCompiler{Compiler: NewCompiler(), query: q}
}

// override registerParameter to use the query
func (c *QueryIntegratedCompiler) registerParameter(val interface{}) string {
	return c.query.RegisterParameter(val)
}

package parser

type Query struct {
	Clauses []*Clause `@@+`
}

type Clause struct {
	Match  *MatchClause  `  @@`
	Merge  *MergeClause  `| @@`
	Unwind *UnwindClause `| @@`
	Where  *WhereClause  `| @@`
	Set    *SetClause    `| @@`
	Remove *RemoveClause `| @@`
	Return *ReturnClause `| @@`
	Skip   *SkipClause   `| @@`
	Limit  *LimitClause  `| @@`
}

type MatchClause struct {
	Optional bool     `"OPTIONAL"?`
	Pattern  *Pattern `"MATCH" @@`
}

type Pattern struct {
	Variable string `"(" @Ident`
	Label    string `(":" @Ident)? ")"`
}

type WhereClause struct {
	Condition *Condition `"WHERE" @@`
}

type Condition struct {
	Left     *PropertyAccess `@@`
	Operator string          `@(">" | "<" | "=" | ">=" | "<=" | "!=")`
	Right    *Value          `@@`
}

type PropertyAccess struct {
	Variable string `@Ident`
	Property string `"." @Ident`
}

type Value struct {
	String *string `  @String`
	Number *int    `| @Int`
	Param  *string `| @Param`
	List   *List   `| @@`
}

// SkipLimitValue removed

type List struct {
	Elements []*Value `"[" (@@ ("," @@)*)? "]"`
}

type ReturnClause struct {
	Items []*ReturnItem `"RETURN" @@ ("," @@)*`
}

type ReturnItem struct {
	Expression *ReturnExpression `@@`
	Alias      *string           `("AS" @Ident)?`
}

type ReturnExpression struct {
	FunctionCall   *FunctionCall   `@@`
	PropertyAccess *PropertyAccess `| @@`
	MathExpression *MathExpression `| @@`
}

type SimpleTerm struct {
	Parameter *string `@Param`
	Variable  *string `| @Ident`
	Number    *int    `| @Int`
}

type MathExpression struct {
	Left     *MathTerm `@@`
	Operator string    `("+" | "-")?`
	Right    *MathTerm `@@?`
}

type MathTerm struct {
	Parameter *string `@Param`
	Variable  *string `| @Ident`
	Number    *int    `| @Int`
}

type FunctionCall struct {
	Name      string   `@Ident`
	Arguments []*Value `"(" (@@ ("," @@)*)? ")"`
}

type LimitClause struct {
	LimitInt   *int    `  "LIMIT" @Int`
	LimitParam *string `| "LIMIT" @Param`
}

type SkipClause struct {
	SkipInt   *int    `  "SKIP" @Int`
	SkipParam *string `| "SKIP" @Param`
}

type MergeClause struct {
	Pattern *Pattern `"MERGE" @@`
}

type UnwindClause struct {
	Expression *Value `"UNWIND" @@`
	Alias      string `"AS" @Ident`
}

type SetClause struct {
	Assignments []*Assignment `"SET" @@ ("," @@)*`
}

type Assignment struct {
	PropertyAccess *PropertyAccess `@@`
	Value          *Value          `"=" @@`
}

type RemoveClause struct {
	Properties []*PropertyAccess `"REMOVE" @@ ("," @@)*`
}

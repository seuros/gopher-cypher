package parser

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/seuros/gopher-cypher/src/cypher"
)

var cypherLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "String", Pattern: `"[^"]*"`},
	{Name: "Param", Pattern: `\$[a-zA-Z_][a-zA-Z0-9_]*`}, // Added Param rule
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "Int", Pattern: `\d+`},
	{Name: "Operators", Pattern: `>=|<=|!=|>|<|=`},
	{Name: "Punct", Pattern: `[(),.:\[\]\+\-]`}, // Removed $ from Punct
	{Name: "whitespace", Pattern: `\s+`},
})

type Parser struct {
	parser *participle.Parser[Query]
}

func New() (*Parser, error) {
	parser, err := participle.Build[Query](
		participle.Lexer(cypherLexer),
		participle.Unquote("String"),
		participle.CaseInsensitive("MATCH", "WHERE", "RETURN", "LIMIT", "SKIP", "OPTIONAL", "MERGE", "UNWIND", "AS", "SET", "REMOVE"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build parser: %w", err)
	}

	return &Parser{parser: parser}, nil
}

func (p *Parser) Parse(input string) (*cypher.Query, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}

	query, err := p.parser.ParseString("", input)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return convertToAST(query)
}

func validateInput(input string) error {
	if strings.Contains(input, ";") {
		return fmt.Errorf("multiple statements not allowed")
	}

	if strings.Contains(input, "'") {
		return fmt.Errorf("single quotes not allowed, use double quotes")
	}

	return nil
}

func convertToAST(query *Query) (*cypher.Query, error) {
	q := cypher.NewQuery()

	for _, clause := range query.Clauses {
		if clause.Match != nil {
			pattern := fmt.Sprintf("(%s", clause.Match.Pattern.Variable)
			if clause.Match.Pattern.Label != "" {
				pattern += ":" + clause.Match.Pattern.Label
			}
			pattern += ")"
			
			matchNode := &cypher.MatchNode{Pattern: pattern}
			q.AddClause(cypher.NewClauseAdapter(matchNode))
		}

		if clause.Merge != nil {
			pattern := fmt.Sprintf("(%s", clause.Merge.Pattern.Variable)
			if clause.Merge.Pattern.Label != "" {
				pattern += ":" + clause.Merge.Pattern.Label
			}
			pattern += ")"
			
			mergeNode := &cypher.MergeNode{Pattern: pattern}
			q.AddClause(cypher.NewClauseAdapter(mergeNode))
		}

		if clause.Unwind != nil {
			var expression interface{}
			if clause.Unwind.Expression.String != nil {
				expression = *clause.Unwind.Expression.String
			} else if clause.Unwind.Expression.Number != nil {
				expression = *clause.Unwind.Expression.Number
			} else if clause.Unwind.Expression.Param != nil {
				expression = *clause.Unwind.Expression.Param // Removed "$"
			} else if clause.Unwind.Expression.List != nil {
				elements := make([]interface{}, len(clause.Unwind.Expression.List.Elements))
				for i, elem := range clause.Unwind.Expression.List.Elements {
					if elem.String != nil {
						elements[i] = *elem.String
					} else if elem.Number != nil {
						elements[i] = *elem.Number
					} else if elem.Param != nil {
						elements[i] = *elem.Param // Removed "$"
					}
				}
				expression = elements
			}
			
			unwindNode := &cypher.UnwindNode{
				Expression: expression,
				AliasName:  clause.Unwind.Alias,
			}
			q.AddClause(cypher.NewClauseAdapter(unwindNode))
		}

		if clause.Where != nil {
			cond := &cypher.ComparisonExpr{
				LHS: &cypher.PropertyAccessExpr{
					Variable:     &cypher.LiteralExpr{Value: clause.Where.Condition.Left.Variable},
					PropertyName: clause.Where.Condition.Left.Property,
				},
				Op: clause.Where.Condition.Operator,
			}

			if clause.Where.Condition.Right.String != nil {
				cond.RHS = &cypher.LiteralExpr{Value: *clause.Where.Condition.Right.String}
			} else if clause.Where.Condition.Right.Number != nil {
				cond.RHS = &cypher.LiteralExpr{Value: *clause.Where.Condition.Right.Number}
			} else if clause.Where.Condition.Right.Param != nil {
				cond.RHS = &cypher.LiteralExpr{Value: *clause.Where.Condition.Right.Param} // Removed "$"
			}

			whereNode := &cypher.WhereNode{Conditions: []cypher.Expression{cond}}
			q.AddClause(cypher.NewClauseAdapter(whereNode))
		}

		if clause.Set != nil {
			assignments := make([]cypher.SetAssignment, len(clause.Set.Assignments))
			for i, assignment := range clause.Set.Assignments {
				var value interface{}
				if assignment.Value.String != nil {
					value = *assignment.Value.String
				} else if assignment.Value.Number != nil {
					value = *assignment.Value.Number
				} else if assignment.Value.Param != nil {
					value = *assignment.Value.Param // Removed "$"
				}
				
				property := fmt.Sprintf("%s.%s", assignment.PropertyAccess.Variable, assignment.PropertyAccess.Property)
				assignments[i] = &cypher.PropertyAssignment{
					Property: property,
					Value:    value,
				}
			}
			setNode := &cypher.SetNode{Assignments: assignments}
			q.AddClause(cypher.NewClauseAdapter(setNode))
		}

		if clause.Remove != nil {
			items := make([]cypher.RemoveItem, len(clause.Remove.Properties))
			for i, prop := range clause.Remove.Properties {
				property := fmt.Sprintf("%s.%s", prop.Variable, prop.Property)
				items[i] = &cypher.PropertyRemoval{Property: property}
			}
			removeNode := &cypher.RemoveNode{Items: items}
			q.AddClause(cypher.NewClauseAdapter(removeNode))
		}

		if clause.Return != nil {
			items := make([]interface{}, len(clause.Return.Items))
			for i, item := range clause.Return.Items {
				var baseItem interface{}
				
				if item.Expression != nil {
					expr := item.Expression
					if expr.MathExpression != nil {
						leftVal := convertMathTerm(expr.MathExpression.Left)
						
						// Check if this is a full math expression or just a single term
						if expr.MathExpression.Operator != "" && expr.MathExpression.Right != nil {
							rightVal := convertMathTerm(expr.MathExpression.Right)
							baseItem = &cypher.MathExpr{
								Left:     leftVal,
								Operator: expr.MathExpression.Operator,
								Right:    rightVal,
							}
						} else {
							// Just a single term, use it directly
							baseItem = leftVal
						}
					} else if expr.FunctionCall != nil {
						// Convert function arguments
						args := make([]interface{}, len(expr.FunctionCall.Arguments))
						for j, arg := range expr.FunctionCall.Arguments {
							if arg.String != nil {
								args[j] = *arg.String
							} else if arg.Number != nil {
								args[j] = *arg.Number
							} else if arg.Param != nil {
								args[j] = *arg.Param // Removed "$"
							}
						}
						
						baseItem = &cypher.FunctionCallExpr{
							Name:      expr.FunctionCall.Name,
							Arguments: args,
						}
					} else if expr.PropertyAccess != nil {
						baseItem = &cypher.PropertyAccessExpr{
							Variable:     &cypher.LiteralExpr{Value: expr.PropertyAccess.Variable},
							PropertyName: expr.PropertyAccess.Property,
						}
					}
				}
				
				// Handle aliases if present
				if item.Alias != nil && baseItem != nil {
					items[i] = &cypher.AliasExpr{
						Expression: baseItem,
						Alias:      *item.Alias,
					}
				} else {
					items[i] = baseItem
				}
			}
			returnNode := &cypher.ReturnNode{Items: items}
			q.AddClause(cypher.NewClauseAdapter(returnNode))
		}

		if clause.Limit != nil {
			var expressionValue interface{}
			if clause.Limit.LimitInt != nil {
				expressionValue = *clause.Limit.LimitInt
			} else if clause.Limit.LimitParam != nil {
				expressionValue = *clause.Limit.LimitParam // Removed "$"
			}
			limitNode := &cypher.LimitNode{Expression: expressionValue}
			q.AddClause(cypher.NewClauseAdapter(limitNode))
		}

		if clause.Skip != nil {
			var amountValue interface{}
			if clause.Skip.SkipInt != nil {
				amountValue = *clause.Skip.SkipInt
			} else if clause.Skip.SkipParam != nil {
				amountValue = *clause.Skip.SkipParam // Removed "$"
			}
			skipNode := &cypher.SkipNode{Amount: amountValue}
			q.AddClause(cypher.NewClauseAdapter(skipNode))
		}
	}

	return q, nil
}

func convertMathTerm(term *MathTerm) interface{} {
	if term.Parameter != nil {
		return *term.Parameter // Removed "$"
	} else if term.Variable != nil {
		return *term.Variable
	} else if term.Number != nil {
		return *term.Number
	}
	return nil
}
package ast

type Token struct {
	Name string
	Value interface{}
}

type TokenKeyword struct {
	Name string
}


func Keyword(name string, parser parser.Parserer) parser.ParseFunc {
	tok := TokenKeyword{Name:name}
	return parser.ParseFunc(func(state parser.State) parser.State{
		next := parser.Run(state)
		if next.IsError {
			return next
		}
		// modify the result.
		return next.WithResult(tok,next.Index)
	})
}

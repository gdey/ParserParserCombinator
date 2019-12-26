package main

import (
	"errors"
	"fmt"
	"log"
	"unicode"

	"github.com/gdey/ppc/parse"
	"github.com/gdey/ppc/parse/match"
)

type tagged struct {
	Type   string
	Result interface{}
	Index  int64
}

func Tagged(parser parse.Parser, tag string) parse.Parser {
	return parse.Func(func(state parse.State) parse.State {
		nextState := parser.Run(state)
		if nextState.IsError {
			return nextState
		}
		return nextState.WithResult(
			tagged{
				Type:   tag,
				Result: nextState.Result,
				Index:  nextState.Index,
			},
			nextState.Index,
		)
	})
}

func main() {

	quotedString := parse.Between(
		match.String(`"`),
		match.String(`"`),
	)(
		parse.Map(
			match.Until(match.String(`"`))(
				match.Rune(func(_ rune) bool { return true }, nil),
			),
			func(r interface{}) interface{} {
				result, ok := r.([]interface{})
				if !ok {
					return r
				}
				rs := make([]rune, 0, len(result))
				for i := range result {
					iRune, ok := result[i].(rune)
					if !ok {
						continue
					}
					rs = append(rs, iRune)
				}
				return string(rs)
			},
		),
	)
	_ = quotedString
	spaceMatcher := match.Rune(unicode.IsSpace, errors.New("unable to match space"))
	selectMatcher := Tagged(match.StringInsensitive("select"), "SELECT")

	ourParser := parse.SequenceOf(
		selectMatcher,
		Tagged(parse.Many(spaceMatcher), "WHITE-SPACE"),
		parse.Many(
			parse.Map(
				parse.SequenceOf(
					parse.ChoiceOf(
						quotedString,
						match.Letters(),
					),
					parse.Discard(parse.Many(spaceMatcher)),
				),
				func(r interface{}) interface{} {
					log.Printf("transforming: %v ", r)
					results, ok := r.([]interface{})
					if !ok {
						return r
					}
					// expect the interfaces in results to be string, then nil
					// let's check the length
					if len(results) != 2 {
						return r
					}
					str, ok := results[0].(string)
					if !ok {
						return r
					}
					return str
				},
			),
		),
	)

	const corpus = `select this "this is a quoted string"   from table`
	result := parse.String(
		ourParser,
		corpus,
	)

	fmt.Println(corpus)
	fmt.Printf("Result:\n%#v\n", result)

}

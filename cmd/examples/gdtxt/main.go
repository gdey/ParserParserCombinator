package main

import (
	"fmt"

	"github.com/gdey/ppc/lang/gdtxt"
	"github.com/gdey/ppc/parse"
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

	const corpus = `«
		 front-matter
		 | author : Gautam Dey<gautam.dey77@gmail.com>
		 | lang : gdtxt
		 ;
		 This is a description of the document
		 »

	Some text before getting in the meat of the document.

§ This is a level 1 section		

--- first horizontal line

« code
| lang : go
;
package main;

import (
	"fmt"
)

func main(){
	fmt.Printf("Hello world")
}
»

§§ This is a level 2 section		

----- second horizontal line

# This is an ordered list

1. This too is an ordered list

1.1. this is a sub list

•[ ] Checkbox's?

•[x] Go home

• Unordered list item here

This is the \[ this is a parental \] body of the text
This is the [* second *] of the text.
`

	result := parse.String(
		parse.Many(
			parse.ChoiceOf(
				gdtxt.ParseBlock,
				gdtxt.ParseLineTypes,
			),
		),
		corpus,
	)

	fmt.Println(corpus)
	if result.IsError {
		fmt.Printf("Got Error: %v\n", result.Err.Error())
	} else {
		fmt.Printf("Result:\n%#v\n", result.Result)
	}

}

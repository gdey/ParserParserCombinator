package gdtxt

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/gdey/ppc/parse"
	"github.com/gdey/ppc/parse/match"
)

func UntilString(end parse.Parser, body parse.Parser) parse.Parser {
	return parse.Map(
		match.Until(end)(body),
		func(r interface{}) interface{} {
			riface, ok := r.([]interface{})
			if !ok {
				return r
			}
			rbyt := make([]rune, len(riface))
			for i := range riface {
				rbyt[i], ok = riface[i].(rune)
				if !ok {
					panic("All results should be a rune")
				}
			}
			return string(rbyt)
		},
	)
}

// UptoN will attempt to apply parser upto n times
// result is []interface{}
func UptoN(n uint, parser parse.Parser) parse.Parser {
	return parse.Func(func(state parse.State) parse.State {
		if n == 0 {
			return state
		}
		cstate := state
		results := make([]interface{}, 0, int(n))
		for i := 0; i < int(n); i++ {
			nextState := parser.Run(cstate)
			if nextState.IsError {
				if len(results) > 0 {
					return cstate.WithResult(results, cstate.Index)
				}
				return nextState
			}
			results = append(results, nextState.Result)
			cstate = nextState
		}
		return cstate.WithResult(results, cstate.Index)
	})
}

var IgnoreWhiteSpace1 = parse.Discard(parse.Many1(match.Space()))
var IgnoreWhiteSpace = parse.Discard(parse.Many(match.Space()))

func MaybeAsString(r interface{}) interface{} {
	result, ok := r.([]rune)
	if !ok {
		return r
	}
	return string(result)
}

func MaybeAsStringMap(fn func(string) string) func(interface{}) interface{} {
	return func(r interface{}) interface{} {
		result, ok := r.([]rune)
		if !ok {
			return r
		}
		return fn(string(result))
	}
}

type SectionLineLevel uint8

const (
	SectionLineInvalid = SectionLineLevel(iota)
	SectionLineLevelOne
	SectionLineLevelTwo
	SectionLineLevelThree
	SectionLineLevelFour
	SectionLineLevelFive
	SectionLineLevelSix
)

// SectionLine is a line that start with `§` (U+00A7) and is followed by the tile of the line
// There can only be up to six levels, with level one being the primary
// The title will be trimmed of leading and following spaces
type SectionLine struct {
	Level SectionLineLevel // 1-6
	Title string
	Index int64
}

var matchStringTillEndOfLine = parse.Map(
	match.Runes(
		func(r rune) bool { return r != '\n' },
		errors.New("any rune other then '\\n'"),
	),
	MaybeAsStringMap(strings.TrimSpace),
)

var matchLineStart = parse.Discard(
	parse.ChoiceOf(
		parse.StartOfInput(),
		parse.Many(match.String("\n")),
	),
)
var matchLineEnd = parse.Discard(
	parse.SequenceOf(
		match.String("\n"),
		parse.ChoiceOf(
			parse.EndOfInput(),
			match.String("\n"),
		),
	),
)

// MatchLine will match a line
// result is the result of body
func MatchLine(body parse.Parser) parse.Parser {
	return parse.Map(
		parse.SequenceOfNoNil(
			matchLineStart,
			body,
			matchLineEnd,
		),
		func(r interface{}) interface{} {
			result, ok := r.([]interface{})
			if !ok {
				return r
			}
			if len(result) != 1 {
				return r
			}
			return result[0]
		},
	)
}

// Paragraphs are made up of lines
// Lines are made up of words followed by "\n"
// Words are:
//    Whitespace excluding "\n"
//    in-line attributes
//    escaped values \...
//    anything that is not (whitespace, "[", "\", or "\n") (WordLetters)

type WordWhitespace struct {
	Index int64
	Text  string
}

var ParseWordWhitespace = parse.MapIndex(
	match.Runes(func(r rune) bool {
		return r != '\n' && unicode.IsSpace(r)
	},
		errors.New("failed to match non-newline whitespace"),
	),
	func(r interface{}, idx int64) interface{} {
		// know that r is an array of rune
		return WordWhitespace{
			Text:  string(r.([]rune)),
			Index: idx,
		}
	})

type InlineStyle int

const (
	InlineStyleEmphasize InlineStyle = iota
	InlineStyleStrong
	InlineStyleStrikeThrough
	InlineStyleUnderline
	InlineStyleCallout
	InlineStyleQuote
)

type WordInlineStyle struct {
	Style InlineStyle
	// Can be any of the sub words. But we will trim any spaces
	Contents []interface{}
	Index    int64
}

type WordEscaped struct {
	Index int64
	Text  string
}

var ParseWordEscaped = parse.MapIndex(
	parse.SequenceOfNoNil(
		parse.Discard(match.StringInsensitive(`\`)),
		parse.Func(func(state parse.State) parse.State {
			r, n, err := state.ReadNextRune()
			if err != nil {
				return state.WithError(fmt.Errorf("error reading escaped val: %v ", err))
			}
			return state.WithResult(string([]rune{r}), state.Index+int64(n))
		}),
	),
	func(r interface{}, idx int64) interface{} {
		return WordEscaped{
			Index: idx,
			Text:  r.([]interface{})[0].(string),
		}
	},
)

type WordCharacters struct {
	Index int64
	Text  string
}

var ParseWordCharacters = parse.MapIndex(
	match.Runes(func(r rune) bool {
		if unicode.IsSpace(r) {
			return false
		}
		return true
		//return r != '[' && r != '\\'
	},
		errors.New("failed to match word characters"),
	),
	func(r interface{}, idx int64) interface{} {
		return WordCharacters{
			Text:  string(r.([]rune)),
			Index: idx,
		}
	},
)
var ParseWord = parse.ChoiceOf(
	ParseWordWhitespace,
	ParseWordEscaped,
	ParseWordCharacters,
)

var ParsePLine = parse.SequenceOfNoNil(
	parse.Many1(ParseWord),
	parse.Discard(parse.ChoiceOf(
		match.String("\n"),
		parse.EndOfInput(),
	)),
)

var ParseParagraph = parse.SequenceOfNoNil(
	parse.Many1(ParsePLine),
)

var ParseSectionLine = MatchLine(
	parse.MapError(
		parse.MapIndex(
			parse.SequenceOfNoNil(
				parse.Map(
					UptoN(6, match.String("§")),
					func(r interface{}) interface{} {
						result, ok := r.([]interface{})
						if !ok {
							return r
						}
						return SectionLineLevel(len(result))
					},
				),
				IgnoreWhiteSpace,
				matchStringTillEndOfLine,
			),
			func(r interface{}, index int64) interface{} {
				result, ok := r.([]interface{})
				if !ok {
					return r
				}

				level := result[0].(SectionLineLevel)
				title := result[1].(string)
				return SectionLine{
					Index: index,
					Level: level,
					Title: title,
				}
			},
		),
		func(_ parse.State) error {
			return errors.New("Unabled to match a section")
		},
	),
)

// Horizontal line
// Three or more “—” at the start of a line will create a horizontal line.
// Text after the line-markers are ignored.

type HorizontalLine struct {
	Index     int64
	Count     int
	ExtraText string
}

var ParseHLine = MatchLine(
	parse.MapIndex(
		parse.SequenceOfNoNil(
			parse.Discard(match.String("---")),
			parse.Many(match.String("-")),
			matchStringTillEndOfLine,
		),
		func(r interface{}, idx int64) interface{} {
			results := r.([]interface{})
			count := len(results[0].([]interface{})) + 3
			text := results[1].(string)
			return HorizontalLine{
				Index:     idx,
				Count:     count,
				ExtraText: text,
			}
		},
	),
)

// list
type List struct {
	Level     uint // start at zero and goes up
	LevelText []string
	Text      string
	Index     int64
}

var ParseListMarker = parse.ChoiceOf(
	parse.Map(
		parse.SequenceOfNoNil(
			match.String("•["),
			IgnoreWhiteSpace,
			parse.Optional(match.StringInsensitive("X")),
			IgnoreWhiteSpace,
			match.String("]"),
		),
		func(r interface{}) interface{} {
			results := r.([]interface{})
			if len(results) == 3 {
				return "•[X]"
			}
			return "•[]"
		},
	),
	match.String("•"),
	match.String("#"),
	parse.Map(
		parse.SequenceOfNoNil(
			parse.Many1(match.Digit()),
			match.String("."),
		),
		func(r interface{}) interface{} {
			var str strings.Builder
			results := r.([]interface{})
			digits := results[0].([]interface{})
			for i := range digits {
				str.WriteRune(digits[i].(rune))
			}
			str.WriteString(".")
			return str.String()
		},
	),
)

var ParseList = MatchLine(
	parse.MapIndex(
		parse.SequenceOf(
			parse.Many1(ParseListMarker),
			matchStringTillEndOfLine,
		),
		func(r interface{}, idx int64) interface{} {
			results := r.([]interface{})
			lvls := results[0].([]interface{})
			level := uint(len(lvls) - 1)
			lvlText := make([]string, len(lvls))
			for i := range lvls {
				lvlText[i] = lvls[i].(string)
			}
			text := results[1].(string)
			return List{
				Index:     idx,
				Level:     level,
				LevelText: lvlText,
				Text:      text,
			}
		},
	),
)

var ParseLineTypes = parse.ChoiceOf(
	ParseSectionLine,
	ParseHLine,
	ParseList,
	ParseParagraph,
	match.String("\n"),
)

// block parser

type KeyVal struct {
	Key   string
	Val   string
	Index int64
}

type Block struct {
	Type    string
	Headers []KeyVal
	Body    string
	Index   int64
}

var ParseBlockHeader = parse.MapIndex(
	parse.SequenceOf(
		parse.Discard(
			parse.SequenceOf(
				IgnoreWhiteSpace,
				match.String("|"),
			),
		),
		parse.Map(
			parse.SequenceOf(
				IgnoreWhiteSpace,
				match.Letters(),
				IgnoreWhiteSpace,
				parse.ChoiceOf(
					match.String(":"),
					match.String("="),
				),
			),
			func(r interface{}) interface{} {
				result, ok := r.([]interface{})
				if !ok || len(result) < 2 {
					return r
				}
				return result[1]
			},
		),
		parse.Map(
			match.Runes(
				func(r rune) bool {
					return r != ';' && r != '|'
				},
				errors.New("any rune other then ';' or '|'"),
			),
			MaybeAsString,
		),
	),
	func(r interface{}, idx int64) interface{} {
		result, ok := r.([]interface{})
		if !ok || len(result) < 3 {
			return r
		}
		keyString, _ := result[1].(string)
		valString, _ := result[2].(string)
		return KeyVal{
			Key:   keyString,
			Val:   strings.TrimSpace(valString),
			Index: idx,
		}

	},
)

var ParseBlockHeaders = parse.Map(
	parse.Many(ParseBlockHeader),
	func(r interface{}) interface{} {
		result, ok := r.([]interface{})
		if !ok {
			return r
		}
		headers := make([]KeyVal, len(result))
		for i := range result {
			h, _ := result[i].(KeyVal)
			headers[i] = h
		}
		return headers
	},
)

var ParseBlockType = parse.Map(
	match.Runes(func(r rune) bool {
		return unicode.IsLetter(r) ||
			unicode.IsDigit(r) ||
			r == '.' || r == '-' || r == '_'
	},
		errors.New("failed to match identifier"),
	),
	MaybeAsString,
)

var ParseBlock = parse.MapIndex(
	parse.SequenceOf(
		match.String("«"),
		IgnoreWhiteSpace,
		ParseBlockType,
		IgnoreWhiteSpace,
		ParseBlockHeaders,
		parse.MapError(
			match.String(";"),
			func(state parse.State) error {
				idx := 0
				if state.Index > 0 {
					idx = int(state.Index)
				}
				contextBytes := make([]byte, idx+2)
				n, err := state.Source.ReadAt(contextBytes, state.Index)
				ctxString := ""
				if err == nil {
					ctxString = string(contextBytes[:n])

				}
				return fmt.Errorf("Expected to find ';' at %v -- %v", state.Index, ctxString)
			},
		),
		parse.Map(
			match.Runes(func(r rune) bool { return r != '»' }, errors.New("any rune other then '»'")),
			MaybeAsString,
		),
		match.String("»\n"),
	),
	func(r interface{}, idx int64) interface{} {
		results, ok := r.([]interface{})
		if !ok {
			return r
		}
		typ := results[2].(string)
		headers := results[4].([]KeyVal)
		body := results[6].(string)
		return Block{
			Type:    typ,
			Headers: headers,
			Body:    body,
			Index:   idx,
		}
	},
)

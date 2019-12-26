package parse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode/utf8"
)

type Parser interface {
	Run(State) State
}

type Func func(State) State

func (fn Func) Run(state State) State {
	if state.IsError {
		return state
	}
	return fn(state)
}

type State struct {
	Result interface{}
	Index  int64
	Source io.ReaderAt

	IsError bool
	Err     error
}

func (state State) WithResult(Result interface{}, Index int64) State {
	return State{
		Result: Result,
		Index:  Index,
		Source: state.Source,

		IsError: state.IsError,
		Err:     state.Err,
	}
}

func (state State) WithError(err error) State {
	return State{
		Result: state.Result,
		Index:  state.Index,
		Source: state.Source,

		IsError: true,
		Err:     err,
	}
}

// LineOffset returns the line (as defined by "\n") and offset of the currect index
func (state State) LineOffset() (line int, offset int, err error) {
	var (
		buff = make([]byte, state.Index)
		n    int
	)
	n, err = state.Source.ReadAt(buff, 0)
	if n > 0 {
		line = bytes.Count(buff[:n], []byte("\n"))
		offset = n - bytes.LastIndex(buff[:n], []byte("\n"))
	}
	return line, offset, err
}

func (state State) ReadNextBytes(n int) ([]byte, int, error) {
	buff := make([]byte, n)
	nn, err := state.Source.ReadAt(buff, state.Index)
	return buff, nn, err
}

func (state State) ReadNextRune() (rune, int, error) {
	var (
		buff []byte
		err  error
	)
	for byteCount, gotFullRune := 1, false; !gotFullRune; byteCount, gotFullRune = byteCount+1, utf8.FullRune(buff) {
		buff, _, err = state.ReadNextBytes(byteCount)
		if err != nil {
			return 0, 0, err
		}
	}
	r, n := utf8.DecodeRune(buff)
	return r, n, nil
}

func (state State) ReadNextRunes(n int) ([]rune, int, error) {
	var (
		bytesRead = 0
		runesRead = make([]rune, 0, n)
		// Make a copy of the state, that we will modify.
		cstate = state
	)

	for i := 0; i < n; i++ {
		r, nn, err := cstate.ReadNextRune()
		if err != nil {
			return runesRead, bytesRead, err
		}
		bytesRead += nn
		runesRead = append(runesRead, r)
		cstate.Index += int64(nn)
	}
	return runesRead, bytesRead, nil
}

func ApplyN(n int, parser Parser) Parser {
	return Func(func(state State) State {
		var (
			results []interface{}
			next    = state
		)
		for i := 0; i < n; i++ {
			next = parser.Run(next)
			if next.IsError {
				return state.WithError(fmt.Errorf("failed to match %v times", n))
			}
			results = append(results, next.Result)
		}

		return state.WithResult(
			results,
			next.Index,
		)
	})
}

func Between(left, right Parser) func(Parser) Parser {
	return func(content Parser) Parser {
		return Map(
			SequenceOf(
				left,
				content,
				right,
			),
			func(r interface{}) interface{} {
				results, ok := r.([]interface{})
				if !ok {
					return r
				}
				if len(results) < 2 {
					return r
				}
				// Content should be in the middle
				return results[1]
			},
		)
	}
}

func Chain(parser Parser, fn func(result interface{}) Parser) Parser {
	return Func(func(state State) State {

		nextState := parser.Run(state)
		if nextState.IsError {
			return nextState
		}

		p := fn(nextState.Result)
		return p.Run(nextState)
	})
}

// ChoiceOf will select the first parser that matches
func ChoiceOf(parser1 Parser, rest ...Parser) Parser {
	return Func(func(state State) State {
		next := parser1.Run(state)
		if !next.IsError {
			return next
		}
		for _, p := range rest {
			next = p.Run(state)
			if !next.IsError {
				return next
			}

		}
		return state.WithError(errors.New("did not match any choice"))
	})
}

func MapIndex(parser Parser, fn func(result interface{}, index int64) interface{}) Parser {
	return Func(func(state State) State {

		nextState := parser.Run(state)
		if nextState.IsError {
			return nextState
		}
		// Need to modify the result.
		return nextState.WithResult(
			fn(nextState.Result, state.Index),
			nextState.Index,
		)
	})
}
func Map(parser Parser, fn func(result interface{}) interface{}) Parser {
	return Func(func(state State) State {

		nextState := parser.Run(state)
		if nextState.IsError {
			return nextState
		}
		// Need to modify the result.
		return nextState.WithResult(
			fn(nextState.Result),
			nextState.Index,
		)
	})
}

func MapError(parser Parser, fn func(state State) error) Parser {
	return Func(func(state State) State {
		nextState := parser.Run(state)
		if !nextState.IsError {
			return nextState
		}
		// Need to modify the error.
		return nextState.WithError(
			fn(nextState),
		)
	})
}

// Discard will discard the result of the match, if there was a match
func Discard(parser Parser) Parser {
	return Map(parser, func(_ interface{}) interface{} { return nil })
}

// Many will match zero or more of the given parser
// Many will not error
func Many(parser Parser) Parser {
	return Func(func(state State) State {
		var (
			results []interface{}
			next    State
		)
		for {
			next = parser.Run(state)
			if next.IsError {
				break
			}
			results = append(results, next.Result)
			state = next
		}

		return state.WithResult(
			results,
			state.Index,
		)
	})
}

// Many1 will match at least once
func Many1(parser Parser) Parser {
	return Func(func(state State) State {
		var (
			results []interface{}
			next    State
		)
		for {
			next = parser.Run(state)
			if next.IsError {
				break
			}
			results = append(results, next.Result)
			state = next
		}
		if len(results) >= 1 {
			return state.WithResult(
				results,
				state.Index,
			)
		}
		return state.WithError(errors.New("failed to match at least once"))
	})
}

// Optional will attempt to apply the given parser but if it errors, it will
// return nil and not error
func Optional(parser Parser) Parser {
	return Func(func(state State) State {
		next := parser.Run(state)
		if next.IsError {
			return state
		}
		return next
	})
}

// Peek will see if the parser would match.
// It returns the nil without modifying the state
// Otherwise it returns an error
// return nil and not error
func Peek(parser Parser) Parser {
	return Func(func(state State) State {
		next := parser.Run(state)
		if next.IsError {
			return state.WithError(errors.New("would not match"))
		}
		return state
	})
}

// SequenceOf will attempt to match each given parser in the order specified
func SequenceOf(parser1 Parser, rest ...Parser) Parser {
	return Func(func(state State) State {

		next := parser1.Run(state)
		if next.IsError {
			return next
		}

		results := []interface{}{
			next.Result,
		}

		for _, p := range rest {
			next = p.Run(next)
			if next.IsError {
				return state.WithError(next.Err)
			}
			results = append(results, next.Result)
		}
		return state.WithResult(
			results,
			next.Index,
		)
	})
}

// SequenceOfNoNil will attempt to match each given parser in the order specified
func SequenceOfNoNil(parser1 Parser, rest ...Parser) Parser {
	return Func(func(state State) State {

		next := parser1.Run(state)
		if next.IsError {
			return next
		}

		results := []interface{}{}
		if next.Result != nil {
			results = append(results, next.Result)
		}

		for _, p := range rest {
			next = p.Run(next)
			if next.IsError {
				return state.WithError(next.Err)
			}
			if next.Result == nil {
				continue
			}
			results = append(results, next.Result)
		}
		return state.WithResult(
			results,
			next.Index,
		)
	})
}

func StartOfInput() Parser {
	return Func(func(state State) State {
		if state.Index != 0 {
			return state.WithError(errors.New("expected start of input"))

		}
		return state
	})
}
func EndOfInput() Parser {
	return Func(func(state State) State {
		buff := make([]byte, 1)
		//force read 1 byte passed to see if we get EOF
		_, err := state.Source.ReadAt(buff, state.Index+1)
		if err != io.EOF {
			return state.WithError(errors.New("expected end of input"))
		}
		return state
	})
}

func StartOfLine() Parser {
	return Func(func(state State) State {
		if state.Index == 0 {
			return state
		}

		// check to see if the previous index was -1
		buff := make([]byte, 1)
		_, err := state.Source.ReadAt(buff, state.Index-1)
		log.Printf("buff: %c -- %v", buff[0], err)
		if err != nil || buff[0] != '\n' {
			return state.WithError(errors.New("expected start of line"))
		}

		return state
	})
}

// Parse helpers

func String(parser Parser, s string) State {
	originalState := State{
		Source: strings.NewReader(s),
	}
	return parser.Run(originalState)
}

func File(parser Parser, filename string) (State, error) {
	f, err := os.Open(filename)
	if err != nil {
		return State{}, err
	}
	defer f.Close()

	originalState := State{
		Source: f,
	}
	return parser.Run(originalState), nil
}

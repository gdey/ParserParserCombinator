package match

import (
	"bytes"
	"errors"
	"fmt"
	"unicode"

	"github.com/gdey/ppc/parse"
)

// AnyRune will match one rune
func AnyRune() parse.Parser {
	return Rune(func(_ rune) bool { return true }, errors.New("unable to match letter"))
}

// Digit matches one unicode digit
// result is a rune
func Digit() parse.Parser {
	return parse.Func(func(state parse.State) parse.State {

		r, n, err := state.ReadNextRune()
		if err != nil || !unicode.IsDigit(r) {
			return state.WithError(
				errors.New("unable to match letter"),
			)
		}
		return state.WithResult(r, state.Index+int64(n))
	})
}

// Letter matches one unicode letter
// result is a rune
func Letter() parse.Parser {
	return Rune(unicode.IsLetter, errors.New("unable to match letter"))
}

// Letters matches one or more letters
// result is a string
func Letters() parse.Parser {

	return parse.Map(
		Runes(unicode.IsLetter, errors.New("failed to match any letters")),
		func(r interface{}) interface{} {
			result, ok := r.([]rune)
			if !ok {
				return r
			}
			return string(result)
		},
	)

}

// Rune matches as rune described by the provided function
// result is a rune
func Rune(fn func(rune) bool, errVal error) parse.Parser {
	return parse.Func(func(state parse.State) parse.State {

		r, n, err := state.ReadNextRune()
		if err != nil || !fn(r) {
			return state.WithError(errVal)
		}
		return state.WithResult(r, state.Index+int64(n))
	})
}

func RuneN(n int) parse.Parser {
	return parse.Func(func(state parse.State) parse.State {
		var (
			results = make([]rune, n)
			cstate  = state
		)

		for i := 0; i < n; i++ {
			r, nn, err := cstate.ReadNextRune()
			if err != nil {
				return state.WithError(fmt.Errorf("unable to match %v runes", n))
			}
			results[i] = r
			cstate.Index += int64(nn)
		}

		return state.WithResult(results, cstate.Index)
	})
}

// Runes matches one or more runes described by the provided function
// results in an array of runes
func Runes(fn func(rune) bool, errVal error) parse.Parser {
	return parse.Func(func(state parse.State) parse.State {
		var (
			runesRead []rune
			// Make a copy of the state, that we will modify.
			cstate = state
		)

		for {

			r, n, err := cstate.ReadNextRune()
			if err != nil || !fn(r) {

				if len(runesRead) >= 1 {
					return state.WithResult(
						runesRead,
						cstate.Index,
					)
				}
				return state.WithError(errVal)
			}

			runesRead = append(runesRead, r)
			cstate.Index += int64(n)
		}

	})
}

// Space matches one space
func Space() parse.Parser {
	return Rune(unicode.IsSpace, errors.New("unable to match a space"))
}

// String matches a string exactly
func String(match string) parse.Parser {
	matchBytes := []byte(match)
	return parse.Func(func(state parse.State) parse.State {

		buff, n, err := state.ReadNextBytes(len(matchBytes))

		if err != nil {
			return state.WithError(
				fmt.Errorf("unable to match %v", match),
			)
		}
		if !bytes.HasPrefix(matchBytes, buff) {
			return state.WithError(
				fmt.Errorf("unable to match %v", match),
			)
		}
		return state.WithResult(
			match,
			state.Index+int64(n),
		)

	})
}

// StringInsensitive matches a string insensitive to the casing
func StringInsensitive(match string) parse.Parser {
	matchBytes := bytes.ToUpper([]byte(match))
	return parse.Func(func(state parse.State) parse.State {

		buff, n, err := state.ReadNextBytes(len(matchBytes))

		if err != nil {
			return state.WithError(
				fmt.Errorf("unable to match %v", match),
			)
		}
		if !bytes.HasPrefix(matchBytes, bytes.ToUpper(buff)) {
			return state.WithError(
				fmt.Errorf("unable to match %v", match),
			)
		}
		return state.WithResult(
			match,
			state.Index+int64(n),
		)

	})
}

// Until will apply the body parser until the end parser matches.
// State will be left at end parser
// result is []interface{}
func Until(end parse.Parser) func(parse.Parser) parse.Parser {
	return func(body parse.Parser) parse.Parser {
		return parse.Func(func(state parse.State) parse.State {

			var results []interface{}

			cstate := state
			for {

				// Check until condition
				nextState := end.Run(cstate)
				if !nextState.IsError {
					// We need to stop.
					return cstate.WithResult(results, cstate.Index)
				}

				// run the body parser.
				cstate = body.Run(cstate)
				if cstate.IsError {
					// Return the error.
					return nextState
				}
				results = append(results, cstate.Result)
			}
		})
	}
}

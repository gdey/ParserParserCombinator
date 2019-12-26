/*
Package escapedString allows one to parse escaped strings which is a string surrounded by `"` and has 
a `\` to escape characters:
\" -> "
\n -> newline
\\ -> \
\ua2f3 -> unicode character where the hex number is the utf8 code point
package escapedString



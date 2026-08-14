package main

import (
	"fmt"
	"io"
	"os"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// highlightSQL applies syntax highlighting to SQL string using Chroma
func highlightSQL(w io.Writer, sql string) {
	// If the output is not a terminal, just print the raw SQL
	if !isTerminal(w) {
		fmt.Fprint(w, sql)
		return
	}

	lexer := lexers.Get("sql")
	if lexer == nil {
		lexer = lexers.Fallback
	}
	style := styles.Get("monokai") // You can change this to "dracula", "solarized-dark", etc.
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, sql)
	if err != nil {
		fmt.Fprint(w, sql)
		return
	}

	err = formatter.Format(w, style, iterator)
	if err != nil {
		fmt.Fprint(w, sql)
	}
}

// isTerminal checks if the given writer is a terminal
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err == nil {
			return (stat.Mode() & os.ModeCharDevice) != 0
		}
	}
	return false
}

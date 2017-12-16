package main

import (
	"github.com/alecthomas/participle"
)

type Grammar struct {
	Entities []*ConfigEntity `{ @@ }`
}

type ConfigEntity struct {
	Type   string   `@Ident`
	Name   string   `@String`
	Routes []*Route `["{" { @@ } "}"]`
}

type Route struct {
	Selector string `@String '-' '>'`
	Type     string `@Ident`
	Name     string `@String`
}

func parseConfig(text string) (*Grammar, error) {
	parser, err := participle.Build(&Grammar{}, nil)
	if err != nil {
		return nil, err
	}

	grammar := &Grammar{}
	err = parser.ParseString(text, grammar)
	if err != nil {
		return nil, err
	}

	return grammar, nil
}

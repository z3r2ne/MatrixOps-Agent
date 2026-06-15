package config

import (
	"bytes"
	"encoding/json"
)

func parseJSONC(input []byte, out interface{}) error {
	trimmed := stripComments(input)
	trimmed = stripTrailingCommas(trimmed)
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func stripComments(input []byte) []byte {
	var out []byte
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		if inLineComment {
			if c == '\n' {
				inLineComment = false
				out = append(out, c)
			}
			continue
		}
		if inBlockComment {
			if c == '*' && i+1 < len(input) && input[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(input) && input[i+1] == '/' {
			inLineComment = true
			i++
			continue
		}
		if c == '/' && i+1 < len(input) && input[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}
		out = append(out, c)
	}
	return out
}

func stripTrailingCommas(input []byte) []byte {
	var out []byte
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(input) && (input[j] == ' ' || input[j] == '\n' || input[j] == '\t' || input[j] == '\r') {
				j++
			}
			if j < len(input) && (input[j] == '}' || input[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

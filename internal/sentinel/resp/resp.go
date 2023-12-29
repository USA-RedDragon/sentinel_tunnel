// Package resp provides helpers to build Redis serialization protocol strings
// according to https://redis.io/docs/reference/protocol-spec/
package resp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel/resp/token"
)

type Command []fmt.Stringer

func (c Command) String() string {
	var result strings.Builder
	for _, val := range c {
		result.WriteString(val.String())
	}
	return result.String()
}

// Array builds a RESP array
// See https://redis.io/docs/reference/protocol-spec/#arrays
type Array []fmt.Stringer

func (a Array) String() string {
	var result strings.Builder
	for _, val := range a {
		result.WriteString(val.String())
	}
	return string(token.Array) + strconv.Itoa(len(a)) + token.EOL + result.String()
}

// SimpleError builds a simple error
// See https://redis.io/docs/reference/protocol-spec/#simple-errors
type SimpleError string

func (s SimpleError) String() string {
	return string(token.SimpleError) + string(s) + token.EOL
}

// SimpleString builds a simple string
// See https://redis.io/docs/reference/protocol-spec/#simple-strings
type SimpleString string

func (s SimpleString) String() string {
	return string(token.SimpleString) + string(s) + token.EOL
}

// BulkString builds a binary string
// See https://redis.io/docs/reference/protocol-spec/#bulk-strings
type BulkString string

func (b BulkString) String() string {
	return string(token.BulkString) + strconv.Itoa(len(b)) + token.EOL + string(b) + token.EOL
}

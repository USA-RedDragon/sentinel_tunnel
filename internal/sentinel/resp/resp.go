// Package resp provides helpers to build Redis serialization protocol strings
// according to https://redis.io/docs/reference/protocol-spec/
package resp

import (
	"fmt"
	"strconv"
	"strings"
)

type Command interface {
	fmt.Stringer
}

// Array builds a RESP array
// See https://redis.io/docs/reference/protocol-spec/#arrays
type Array []Command

func (a Array) String() string {
	var result strings.Builder
	for _, val := range a {
		result.WriteString(val.String())
	}
	return "*" + strconv.Itoa(len(a)) + "\r\n" + result.String()
}

// BulkString builds a binary string
// See https://redis.io/docs/reference/protocol-spec/#bulk-strings
type BulkString string

func (b BulkString) String() string {
	return "$" + strconv.Itoa(len(b)) + "\r\n" + string(b) + "\r\n"
}

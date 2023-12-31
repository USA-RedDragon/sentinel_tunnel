package resp

import (
	"fmt"
	"testing"
)

func TestRespCommand(t *testing.T) {
	t.Parallel()

	var cmd Command
	cmd = append(cmd, Array{
		BulkString("auth"),
		BulkString("password"),
	})

	if cmd.String() != "*2\r\n$4\r\nauth\r\n$8\r\npassword\r\n" {
		t.Error("Command.String() did not return expected string")
	}
}

func TestRespSimpleErrorEncode(t *testing.T) {
	t.Parallel()

	serr := SimpleError("Testing Simple Error").String()
	if serr != "-Testing Simple Error\r\n" {
		t.Error("SimpleError.String() did not return expected string")
	}
}

func TestRespSimpleStringEncode(t *testing.T) {
	t.Parallel()

	sstr := SimpleString("Testing Simple String").String()
	if sstr != "+Testing Simple String\r\n" {
		t.Error("SimpleString.String() did not return expected string")
	}
}

func BenchmarkRespSimpleErrorEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		testErr := fmt.Errorf("Testing Simple Error %d", i)
		b.StartTimer()
		_ = SimpleError(testErr.Error()).String()
	}
}

func BenchmarkRespSimpleStringEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		testStr := fmt.Sprintf("Testing Simple String %d", i)
		b.StartTimer()
		_ = SimpleString(testStr).String()
	}
}

package redisstore

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadBulkNil(t *testing.T) {
	_, err := readReply(bufio.NewReader(strings.NewReader("$-1\r\n")))
	if err != ErrMissing {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
}

func TestReadBulkString(t *testing.T) {
	got, err := readReply(bufio.NewReader(strings.NewReader("$2\r\n42\r\n")))
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if got != "42" {
		t.Fatalf("got %v, want 42", got)
	}
}

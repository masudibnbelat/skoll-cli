// Location: new_test.go
package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestAskPackageAutoFill(t *testing.T) {
	// A bare name gets the com.example. prefix; upper case is
	// lowered; a full package is used as-is.
	cases := []struct{ in, want string }{
		{"demo", "com.example.demo"},
		{"Demo", "com.example.demo"},
		{"  demo  ", "com.example.demo"},
		{"com.masud.moan", "com.masud.moan"},
		{"com.example.test", "com.example.test"},
		{"my.app", "my.app"},
	}
	for _, c := range cases {
		newPackage = c.in
		got := askPackage(bufio.NewReader(strings.NewReader("")), "test")
		if got != c.want {
			t.Fatalf("askPackage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	newPackage = ""

	// The interactive prompt: typing just "demo" fills the rest.
	reader := bufio.NewReader(strings.NewReader("demo\n"))
	if got := askPackage(reader, "test"); got != "com.example.demo" {
		t.Fatalf("prompted askPackage = %q, want com.example.demo", got)
	}
	// Empty answer takes the default.
	reader = bufio.NewReader(strings.NewReader("\n"))
	if got := askPackage(reader, "test"); got != "com.example.test" {
		t.Fatalf("empty prompt askPackage = %q, want com.example.test", got)
	}
}

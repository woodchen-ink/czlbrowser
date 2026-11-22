//go:build windows

package dpapi

import "testing"

func TestRoundTrip(t *testing.T) {
	in := []byte("secret-bytes")
	enc, err := Protect(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Unprotect(enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(in) {
		t.Fatalf("got %q", out)
	}
}

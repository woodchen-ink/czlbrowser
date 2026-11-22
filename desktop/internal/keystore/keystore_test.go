package keystore

import (
	"context"
	"errors"
	"testing"

	"github.com/woodchen-ink/go-web-utils/syncsdk"
)

func TestStoreRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	s.protect = func(b []byte) ([]byte, error) { return append([]byte("x"), b...), nil }
	s.unprotect = func(b []byte) ([]byte, error) { return b[1:], nil }
	ctx := context.Background()
	if _, err := s.Get(ctx, "a/user-key"); !errors.Is(err, syncsdk.ErrKeyNotFound) {
		t.Fatalf("want ErrKeyNotFound, got %v", err)
	}
	if err := s.Put(ctx, "a/user-key", []byte("k1")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "a/user-key")
	if err != nil || string(got) != "k1" {
		t.Fatalf("got %q %v", got, err)
	}
	if err := s.Delete(ctx, "a/user-key"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "a/user-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "a/user-key"); !errors.Is(err, syncsdk.ErrKeyNotFound) {
		t.Fatalf("want ErrKeyNotFound after delete, got %v", err)
	}
}

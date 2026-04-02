package hash

import "testing"

func TestHash(t *testing.T) {
	// Known SHA-256 of "hello"
	h := Hash([]byte("hello"))
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if h != expected {
		t.Errorf("expected %s, got %s", expected, h)
	}
}

func TestHashStable(t *testing.T) {
	content := []byte("test content for stability check")
	h1 := Hash(content)
	h2 := Hash(content)
	if h1 != h2 {
		t.Error("hash not stable across calls")
	}
}

func TestHashEmpty(t *testing.T) {
	h := Hash([]byte{})
	// SHA-256 of empty string
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if h != expected {
		t.Errorf("expected %s, got %s", expected, h)
	}
}

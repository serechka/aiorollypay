package uuid

import (
	"regexp"
	"testing"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewV4Format(t *testing.T) {
	for i := 0; i < 1000; i++ {
		id := NewV4()
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("NewV4() = %q, want canonical lowercase UUID v4", id)
		}
	}
}

func TestNewV4Unique(t *testing.T) {
	seen := make(map[string]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id := NewV4()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate UUID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func BenchmarkNewV4(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewV4()
	}
}

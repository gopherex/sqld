package nodeid

import "testing"

func TestNodeIDPath(t *testing.T) {
	b := New("stmt0")
	if got := b.Child("select").Child("where").String(); got != "stmt0/select/where" {
		t.Fatalf("got %q", got)
	}
	if got := b.Child("from").Index(1).String(); got != "stmt0/from:1" {
		t.Fatalf("got %q", got)
	}
}

func TestNodeIDImmutable(t *testing.T) {
	b := New("root")
	_ = b.Child("a")
	_ = b.Child("b")
	if got := b.String(); got != "root" {
		t.Fatalf("base mutated: %q", got)
	}
}

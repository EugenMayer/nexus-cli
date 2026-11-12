package keychain

import (
	"slices"
	"testing"
)

func TestList(t *testing.T) {
	t.Cleanup(MockInit())
	kc := New("app")

	if names, err := kc.List("github.aliases"); err != nil || names != nil {
		t.Fatalf("List() of missing list = %v, %v", names, err)
	}
	for _, n := range []string{"geek-labs", "default", "geek-labs"} {
		if err := kc.AddToList("github.aliases", n); err != nil {
			t.Fatal(err)
		}
	}
	if names, _ := kc.List("github.aliases"); !slices.Equal(names, []string{"default", "geek-labs"}) {
		t.Errorf("List() = %v", names)
	}

	_ = kc.RemoveFromList("github.aliases", "default")
	if names, _ := kc.List("github.aliases"); !slices.Equal(names, []string{"geek-labs"}) {
		t.Errorf("List() after remove = %v", names)
	}
	_ = kc.RemoveFromList("github.aliases", "geek-labs")
	if _, err := kc.Get("github.aliases"); err == nil {
		t.Error("empty list should be deleted")
	}
}

package nexus

import "testing"

func TestRegexpMatcher(t *testing.T) {
	m, err := NewMatcher([]string{`^logs/2024-\d{2}/.*\.log$`, `^tmp/.*\.tar\.gz$`})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"logs/2024-01/app.log":     true,
		"logs/2024-12/sub/x.log":   true,
		"logs/2024-1/app.log":      false,
		"logs/2023-01/app.log":     false,
		"logs/2024-01/app.txt":     false,
		"tmp/backup.tar.gz":        true,
		"tmp/nested/backup.tar.gz": true,
		"other/file":               false,
	}
	for path, want := range cases {
		if got := m.Match(path); got != want {
			t.Errorf("Match(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestNewMatcherErrors(t *testing.T) {
	if _, err := NewMatcher(nil); err == nil {
		t.Error("expected error for empty patterns")
	}
	if _, err := NewMatcher([]string{"("}); err == nil {
		t.Error("expected error for invalid regexp")
	}
}

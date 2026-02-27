package feature_flags

import "testing"

func TestNewService(t *testing.T) {
	s := NewService()
	if (s == Service{}) {
		return
	}
	t.Fatalf("unexpected service value: %#v", s)
}

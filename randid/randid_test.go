package randid

import (
	"testing"
)

func TestRandID(t *testing.T) {
	for i := 0; i < 10; i++ {
		id := New()
		t.Log(id)
		t.Log(Decode(id))
	}
}

func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Decode("AH3WD6LLNNV24Z5O")
	}
}

func BenchmarkRandID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New()
	}
}

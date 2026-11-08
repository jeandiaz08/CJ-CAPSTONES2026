package application

import (
	"strings"
	"testing"
)

func TestTemporaryPassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		p, err := TemporaryPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != tempLen {
			t.Fatalf("largo %d", len(p))
		}
		if !strings.ContainsAny(p, tempUpper) || !strings.ContainsAny(p, tempLower) || !strings.ContainsAny(p, tempDigits) {
			t.Fatalf("no cumple la política del realm (mayúscula, minúscula y número): %q", p)
		}
		if strings.ContainsAny(p, "0O1lI") {
			t.Fatalf("tiene caracteres confusos: %q", p)
		}
		if seen[p] {
			t.Fatalf("se repitió %q", p)
		}
		seen[p] = true
	}
}

package domain

import (
	"strconv"
	"strings"
)

// NormalizeRUT valida un RUT chileno (módulo 11) y lo devuelve con puntos y guion
// (76.123.456-7). Acepta el número con o sin puntos y guion, y la K mayúscula o minúscula.
func NormalizeRUT(raw string) (string, bool) {
	clean := strings.ToUpper(strings.NewReplacer(".", "", "-", "", " ", "").Replace(raw))
	if len(clean) < 2 || len(clean) > 9 {
		return "", false
	}
	body, dv := clean[:len(clean)-1], clean[len(clean)-1:]
	n, err := strconv.Atoi(body)
	if err != nil || n < 1_000_000 {
		return "", false
	}
	if checkDigit(n) != dv {
		return "", false
	}
	// Puntos cada tres dígitos desde la derecha.
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String() + "-" + dv, true
}

func checkDigit(n int) string {
	sum, mul := 0, 2
	for ; n > 0; n /= 10 {
		sum += (n % 10) * mul
		if mul++; mul > 7 {
			mul = 2
		}
	}
	switch r := 11 - sum%11; r {
	case 11:
		return "0"
	case 10:
		return "K"
	default:
		return strconv.Itoa(r)
	}
}

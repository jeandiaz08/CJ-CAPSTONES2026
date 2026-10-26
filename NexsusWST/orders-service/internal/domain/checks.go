package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Comuna por similitud (H-07 T04)
// ---------------------------------------------------------------------------

// NormalizeName quita tildes, mayúsculas y espacios repetidos.
func NormalizeName(s string) string {
	repl := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n")
	s = repl.Replace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// MatchCommune busca la comuna en el catálogo. Devuelve el nombre oficial si
// coincide (sin importar tildes/mayúsculas); si no, una sugerencia cercana.
func MatchCommune(input string, catalog []string) (official string, suggestion string) {
	in := NormalizeName(input)
	if in == "" {
		return "", ""
	}
	best, bestDist := "", 1<<30
	for _, c := range catalog {
		n := NormalizeName(c)
		if n == in {
			return c, ""
		}
		if d := levenshtein(in, n); d < bestDist {
			best, bestDist = c, d
		}
	}
	// Sugerir solo si la diferencia es pequeña respecto del largo (errores de tipeo)
	if bestDist <= max(1, len(in)/4) {
		return "", best
	}
	return "", ""
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// ---------------------------------------------------------------------------
// Verificación de empaque (H-12, CU-04)
// ---------------------------------------------------------------------------

// PackingDiscrepancies compara lo contado por Empaque con la orden.
// Vacío = coincide. No se despacha parcialmente (H-11 escenario 3).
func PackingDiscrepancies(expected, counted []Item) []string {
	want := map[string]int{}
	for _, it := range expected {
		want[strings.ToUpper(it.SKU)] += it.Quantity
	}
	got := map[string]int{}
	for _, it := range counted {
		got[strings.ToUpper(strings.TrimSpace(it.SKU))] += it.Quantity
	}
	var out []string
	keys := make([]string, 0, len(want)+len(got))
	for k := range want {
		keys = append(keys, k)
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		w, g := want[k], got[k]
		switch {
		case w == 0:
			out = append(out, fmt.Sprintf("%s no pertenece a la orden (contado %d)", k, g))
		case g < w:
			out = append(out, fmt.Sprintf("Falta %s: esperado %d, contado %d", k, w, g))
		case g > w:
			out = append(out, fmt.Sprintf("Sobra %s: esperado %d, contado %d", k, w, g))
		}
	}
	return out
}

// Package - bulto registrado en empaque (RF-22).
type Package struct {
	ID       int64    `json:"id"`
	Code     string   `json:"code"`
	WeightKg *float64 `json:"weightKg,omitempty"`
	LengthCm *float64 `json:"lengthCm,omitempty"`
	WidthCm  *float64 `json:"widthCm,omitempty"`
	HeightCm *float64 `json:"heightCm,omitempty"`
}

// ValidatePackages exige al menos un bulto y medidas razonables.
func ValidatePackages(p []Package) ValidationErrors {
	var errs ValidationErrors
	if len(p) == 0 {
		errs = append(errs, FieldError{"packages", "registra al menos un bulto"})
	}
	if len(p) > 50 {
		errs = append(errs, FieldError{"packages", "máximo 50 bultos por orden"})
	}
	for i, x := range p {
		for _, v := range []*float64{x.WeightKg, x.LengthCm, x.WidthCm, x.HeightCm} {
			if v != nil && (*v <= 0 || *v > 10000) {
				errs = append(errs, FieldError{fmt.Sprintf("packages[%d]", i), "peso y medidas deben ser mayores que 0"})
				break
			}
		}
	}
	return errs
}

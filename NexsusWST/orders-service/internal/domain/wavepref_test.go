package domain

import "testing"

func TestResolveWavePreference(t *testing.T) {
	cases := []struct {
		name     string
		products []string
		company  string
		want     string
		source   WaveSource
		ok       bool
	}{
		{"un producto con ola gana al cliente", []string{"FRIO"}, "AM", "FRIO", WaveFromProduct, true},
		{"todos los productos con la misma ola", []string{"FRIO"}, "", "FRIO", WaveFromProduct, true},
		{"olas distintas -> cliente", []string{"AM", "FRIO"}, "PM", "PM", WaveFromCompany, true},
		{"olas distintas sin cliente -> reglas", []string{"AM", "FRIO"}, "", "", "", false},
		{"sin ola de producto -> cliente", nil, "AM", "AM", WaveFromCompany, true},
		{"nada -> reglas", nil, "", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ok := ResolveWavePreference(c.products, c.company)
			if ok != c.ok || p.Code != c.want || p.Source != c.source {
				t.Fatalf("got %+v ok=%v", p, ok)
			}
		})
	}
}

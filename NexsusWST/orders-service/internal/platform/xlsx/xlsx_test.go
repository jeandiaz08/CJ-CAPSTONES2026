package xlsx

import (
	"bytes"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]any{
		{"REF-1", "Ñuñoa", 3, "Calle <1> & \"2\""},
		{"REF-2", "", int64(10), nil},
	}
	if err := Write(&buf, "Órdenes", []string{"referencia", "comuna", "cantidad", "nota"}, rows); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFirstSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"referencia", "comuna", "cantidad", "nota"},
		{"REF-1", "Ñuñoa", "3", "Calle <1> & \"2\""},
		{"REF-2", "", "10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("\ngot  %q\nwant %q", got, want)
	}
}

func TestColumns(t *testing.T) {
	for i, name := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 701: "ZZ", 702: "AAA"} {
		if colName(i) != name || colIndex(name+"1") != i {
			t.Errorf("col %d <-> %s", i, name)
		}
	}
}

func TestRejectsGarbage(t *testing.T) {
	if _, err := ReadFirstSheet([]byte("no soy un zip")); err == nil {
		t.Fatal("debía rechazar un archivo que no es xlsx")
	}
}

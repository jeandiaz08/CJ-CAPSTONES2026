// Package xlsx lee y escribe planillas Excel (.xlsx) simples con la
// biblioteca estándar: una hoja, texto y números. Suficiente para la carga
// masiva de órdenes (RF-09) y la exportación del listado (RF-10), sin
// agregar dependencias externas.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxSheetBytes = 50 << 20 // protección contra archivos comprimidos enormes

// ---------------------------------------------------------------------------
// Lectura
// ---------------------------------------------------------------------------

// ReadFirstSheet devuelve las filas de la primera hoja como texto.
func ReadFirstSheet(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("el archivo no es un Excel .xlsx válido")
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	shared, err := readShared(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	sheetPath, err := firstSheetPath(files)
	if err != nil {
		return nil, err
	}
	f := files[sheetPath]
	if f == nil {
		return nil, errors.New("el Excel no tiene hojas")
	}
	raw, err := readZip(f)
	if err != nil {
		return nil, err
	}
	var ws struct {
		Rows []struct {
			R     int `xml:"r,attr"`
			Cells []struct {
				Ref  string `xml:"r,attr"`
				Type string `xml:"t,attr"`
				V    string `xml:"v"`
				IS   struct {
					T string `xml:"t"`
					R []struct {
						T string `xml:"t"`
					} `xml:"r"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(raw, &ws); err != nil {
		return nil, fmt.Errorf("hoja ilegible: %w", err)
	}
	var out [][]string
	for _, row := range ws.Rows {
		var cells []string
		for i, c := range row.Cells {
			col := i
			if c.Ref != "" {
				col = colIndex(c.Ref)
			}
			var val string
			switch c.Type {
			case "s":
				idx, _ := strconv.Atoi(strings.TrimSpace(c.V))
				if idx >= 0 && idx < len(shared) {
					val = shared[idx]
				}
			case "inlineStr":
				val = c.IS.T
				for _, r := range c.IS.R {
					val += r.T
				}
			default: // n, str, b
				val = c.V
				if c.Type == "" || c.Type == "n" {
					val = normalizeNumber(val)
				}
			}
			for len(cells) < col {
				cells = append(cells, "")
			}
			if col < len(cells) {
				cells[col] = val
			} else {
				cells = append(cells, val)
			}
		}
		// filas vacías intermedias se conservan para informar el número correcto
		rowNum := row.R
		if rowNum == 0 {
			rowNum = len(out) + 1
		}
		for len(out) < rowNum-1 {
			out = append(out, nil)
		}
		out = append(out, cells)
	}
	return out, nil
}

// normalizeNumber: "3.0" -> "3" (Excel guarda enteros como decimales).
func normalizeNumber(v string) string {
	if f, err := strconv.ParseFloat(v, 64); err == nil && f == float64(int64(f)) && !strings.ContainsAny(v, "eE") {
		return strconv.FormatInt(int64(f), 10)
	}
	return v
}

func readZip(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxSheetBytes {
		return nil, errors.New("el Excel es demasiado grande")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxSheetBytes))
}

func readShared(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	raw, err := readZip(f)
	if err != nil {
		return nil, err
	}
	var sst struct {
		SI []struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	if err := xml.Unmarshal(raw, &sst); err != nil {
		return nil, err
	}
	out := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		s := si.T
		for _, r := range si.R {
			s += r.T
		}
		out[i] = s
	}
	return out, nil
}

func firstSheetPath(files map[string]*zip.File) (string, error) {
	wb, rels := files["xl/workbook.xml"], files["xl/_rels/workbook.xml.rels"]
	if wb == nil || rels == nil {
		return "xl/worksheets/sheet1.xml", nil
	}
	rawWB, err := readZip(wb)
	if err != nil {
		return "", err
	}
	var w struct {
		Sheets []struct {
			RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(rawWB, &w); err != nil || len(w.Sheets) == 0 {
		return "xl/worksheets/sheet1.xml", nil
	}
	rawRels, err := readZip(rels)
	if err != nil {
		return "", err
	}
	var r struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(rawRels, &r); err != nil {
		return "xl/worksheets/sheet1.xml", nil
	}
	for _, rel := range r.Rels {
		if rel.ID == w.Sheets[0].RID {
			t := strings.TrimPrefix(rel.Target, "/")
			if !strings.HasPrefix(t, "xl/") {
				t = "xl/" + t
			}
			return t, nil
		}
	}
	return "xl/worksheets/sheet1.xml", nil
}

// colIndex: "C7" -> 2
func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
	}
	return n - 1
}

func colName(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// ---------------------------------------------------------------------------
// Escritura
// ---------------------------------------------------------------------------

// Write genera un .xlsx con una hoja: encabezado en negrita y filas de datos.
// Los valores int/int64/float64 se escriben como número; el resto como texto.
func Write(w io.Writer, sheet string, header []string, rows [][]any) error {
	zw := zip.NewWriter(w)
	add := func(name, content string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, content)
		return err
	}
	if sheet == "" {
		sheet = "Hoja1"
	}
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + esc(sheet) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="2"><xf fontId="0" xfId="0"/><xf fontId="1" xfId="0" applyFont="1"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`},
	}
	for _, p := range parts {
		if err := add(p.name, p.body); err != nil {
			return err
		}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetData>`)
	writeRow := func(r int, vals []any, style string) {
		fmt.Fprintf(&b, `<row r="%d">`, r)
		for i, v := range vals {
			ref := colName(i) + strconv.Itoa(r)
			switch x := v.(type) {
			case int, int64, float64:
				fmt.Fprintf(&b, `<c r="%s"%s><v>%v</v></c>`, ref, style, x)
			case nil:
			default:
				fmt.Fprintf(&b, `<c r="%s" t="inlineStr"%s><is><t xml:space="preserve">%s</t></is></c>`, ref, style, esc(fmt.Sprint(x)))
			}
		}
		b.WriteString(`</row>`)
	}
	hdr := make([]any, len(header))
	for i, h := range header {
		hdr[i] = h
	}
	writeRow(1, hdr, ` s="1"`)
	for i, r := range rows {
		writeRow(i+2, r, "")
	}
	b.WriteString(`</sheetData></worksheet>`)
	if err := add("xl/worksheets/sheet1.xml", b.String()); err != nil {
		return err
	}
	return zw.Close()
}

func esc(s string) string {
	var b bytes.Buffer
	// Quita caracteres de control no válidos en XML 1.0
	clean := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 {
			return r
		}
		return -1
	}, s)
	_ = xml.EscapeText(&b, []byte(clean))
	return b.String()
}

// Package courier - adaptadores de couriers y canales (PA-01 Adaptador,
// PA-03 Estrategia: uno por proveedor).
//
// SIMULADO: no llama a ningún courier ni marketplace. Genera una etiqueta PDF
// de prueba rotulada «SIMULACIÓN» y un código de seguimiento ficticio, y
// entrega el detalle de órdenes de Mercado Libre de forma determinística.
// Crear envíos por API depende de PV-08.
package courier

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

type Simulated struct{}

func NewSimulated() *Simulated { return &Simulated{} }

// Label - una página por bulto con remitente, destino y código del bulto.
func (Simulated) Label(ctx context.Context, o domain.Order, packages []domain.Package) ([]byte, string, error) {
	tracking := o.CourierCode
	if tracking == "" {
		tracking = fmt.Sprintf("SIM%s%06d", strings.ToUpper(firstN(o.Courier, 3)), o.ID)
	}
	pages := make([][]string, 0, len(packages))
	for i, p := range packages {
		pages = append(pages, []string{
			"SIMULACION - NO VALIDA PARA DESPACHO",
			"Courier: " + strings.ToUpper(o.Courier) + "   Seguimiento: " + tracking,
			fmt.Sprintf("Bulto %d de %d   Codigo: %s", i+1, len(packages), p.Code),
			"Orden NexusWST #" + fmt.Sprint(o.ID) + "   Ref: " + o.ExternalRef,
			"Destinatario: " + o.RecipientName,
			"Direccion: " + o.AddressLine,
			"Comuna: " + o.Commune,
			"Generada: " + time.Now().Format("2006-01-02 15:04"),
		})
	}
	return simplePDF(pages), tracking, nil
}

func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

// FetchOrder - [SIMULADO] detalle de una orden de Mercado Libre a partir del
// recurso avisado (p. ej. "/orders/2000001234"). Solo lectura.
func (Simulated) FetchOrder(ctx context.Context, integ ports.Integration, resource string) (domain.Draft, error) {
	id := resource[strings.LastIndex(resource, "/")+1:]
	if id == "" {
		return domain.Draft{}, fmt.Errorf("recurso inválido: %s", resource)
	}
	return domain.Draft{
		ExternalRef: "ML-" + id,
		Destination: domain.Destination{RecipientName: "Comprador ML " + id, AddressLine: "Av. Providencia 1234",
			Commune: "Providencia", RegionCode: "RM"},
		DispatchType: "domicilio",
		Items:        []domain.Item{{SKU: "SKU-001", Quantity: 1}},
	}, nil
}

// simplePDF escribe un PDF válido de texto (Helvetica), una página por bloque.
func simplePDF(pages [][]string) []byte {
	var objs []string
	pageIDs := []int{}
	// 1: catálogo, 2: páginas, 3: fuente; luego pares (página, contenido)
	next := 4
	var pageObjs []string
	for _, lines := range pages {
		var c bytes.Buffer
		c.WriteString("BT /F1 12 Tf 40 360 Td 16 TL\n")
		for i, l := range lines {
			if i == 0 {
				c.WriteString("/F1 14 Tf\n")
			}
			fmt.Fprintf(&c, "(%s) Tj T*\n", pdfEscape(l))
			if i == 0 {
				c.WriteString("/F1 12 Tf\n")
			}
		}
		c.WriteString("ET")
		pageID, contentID := next, next+1
		next += 2
		pageIDs = append(pageIDs, pageID)
		pageObjs = append(pageObjs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 420 400] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentID),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", c.Len(), c.String()))
	}
	kids := make([]string, len(pageIDs))
	for i, id := range pageIDs {
		kids[i] = fmt.Sprintf("%d 0 R", id)
	}
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(kids)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	objs = append(objs, pageObjs...)

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// pdfEscape: Helvetica estándar no trae tildes en esta codificación simple;
// se reemplazan para que la etiqueta sea legible.
func pdfEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)",
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ñ", "N")
	out := r.Replace(s)
	return strings.Map(func(c rune) rune {
		if c < 32 || c > 126 {
			return '?'
		}
		return c
	}, out)
}

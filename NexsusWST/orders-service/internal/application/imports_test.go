package application

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/platform/xlsx"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type templateCatalog struct{ ports.CatalogReader }

func (templateCatalog) ActiveCouriers(context.Context) ([]ports.CourierOption, error) {
	return []ports.CourierOption{{Code: "recibelo", Name: "Recíbelo"}, {Code: "starken", Name: "Starken"}}, nil
}
func (templateCatalog) Communes(context.Context) ([]string, error) {
	return []string{"Maipú", "Ñuñoa", "Providencia"}, nil
}
func (templateCatalog) ActiveProducts(_ context.Context, companyID int64) ([]ports.ProductOption, error) {
	if companyID != 7 {
		return nil, nil
	}
	return []ports.ProductOption{{SKU: "AO-1", Description: "Linterna"}, {SKU: "AO-2", Description: "Mochila"}}, nil
}

type importOrders struct {
	ports.OrderRepository
	created []string
}

func (f *importOrders) Create(_ context.Context, o domain.Order, _ string, _ *int64) (domain.Order, error) {
	f.created = append(f.created, o.ExternalRef)
	return o, nil
}

type importFulfillment struct{ ports.FulfillmentRepository }

func (importFulfillment) SaveImport(context.Context, int64, string, int, int, []ports.RowError, string) (int64, error) {
	return 1, nil
}

type silentNotifier struct{ ports.Notifier }

func (silentNotifier) OrderChanged(context.Context, domain.Order) {}

func templateService(orders *importOrders) *Service {
	return NewService(Deps{Catalog: templateCatalog{}, Orders: orders, Fulfillment: importFulfillment{}, Notifier: silentNotifier{}})
}

func admin() tenant.Identity {
	return tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true}}
}

func template(t *testing.T, s *Service, id tenant.Identity, companyID int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := s.ImportTemplate(context.Background(), id, companyID, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipFile(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestImportTemplateListsAndInstructions(t *testing.T) {
	data := template(t, templateService(&importOrders{}), admin(), 7)

	rows, err := xlsx.ReadFirstSheet(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows[0], ",") != strings.Join(ImportColumns, ",") {
		t.Fatalf("la primera hoja debe ser «Órdenes»: %v", rows[0])
	}
	if len(rows) != 3 || !strings.HasPrefix(rows[1][0], ExampleRefPrefix) || rows[1][0] != rows[2][0] {
		t.Fatalf("filas de ejemplo: %v", rows[1:])
	}
	if rows[1][1] != "recibelo" || rows[1][7] != "Ñuñoa" || rows[1][10] != "AO-1" || rows[2][10] != "AO-2" {
		t.Fatalf("el ejemplo debe usar valores válidos: %v", rows[1:])
	}

	orders := zipFile(t, data, "xl/worksheets/sheet1.xml")
	for _, want := range []string{
		`sqref="B2:B5001"><formula1>Instrucciones!$E$2:$E$3</formula1>`,
		`sqref="C2:C5001"><formula1>Instrucciones!$H$2:$H$4</formula1>`,
		`sqref="H2:H5001"><formula1>Instrucciones!$K$2:$K$4</formula1>`,
		`sqref="K2:K5001"><formula1>Instrucciones!$M$2:$M$3</formula1>`,
	} {
		if !strings.Contains(orders, want) {
			t.Errorf("falta la lista %s", want)
		}
	}
	instructions := zipFile(t, data, "xl/worksheets/sheet2.xml")
	for _, want := range []string{"Obligatorio", "cantidad", "recibelo", "retiro_oficina", "Providencia", "AO-2", "Mochila", ExampleRefPrefix} {
		if !strings.Contains(instructions, want) {
			t.Errorf("Instrucciones no menciona %q", want)
		}
	}
	if !strings.Contains(zipFile(t, data, "xl/workbook.xml"), `name="Instrucciones"`) {
		t.Error("falta la hoja Instrucciones")
	}
}

func TestImportTemplateWithoutCompanyHasNoSKUList(t *testing.T) {
	data := template(t, templateService(&importOrders{}), admin(), 0)
	if strings.Contains(zipFile(t, data, "xl/worksheets/sheet1.xml"), `sqref="K2`) {
		t.Fatal("sin empresa no hay lista de SKU")
	}
}

func TestImportTemplateRejectsForeignCompany(t *testing.T) {
	id := tenant.Identity{Profile: tenant.Profile{UserID: 2, Role: tenant.RoleCliente, Companies: []int64{1}}}
	err := templateService(&importOrders{}).ImportTemplate(context.Background(), id, 7, io.Discard)
	if err == nil {
		t.Fatal("no debe entregar los SKU de otra empresa")
	}
}

func TestImportSkipsExampleRows(t *testing.T) {
	orders := &importOrders{}
	s := templateService(orders)
	const csv = "referencia;courier;destinatario;direccion;comuna;sku;cantidad\n" +
		"EJEMPLO-1001;recibelo;Ana;Calle 123;Ñuñoa;AO-1;2\n" +
		"ejemplo-1002;recibelo;Ana;Calle 123;Ñuñoa;AO-1;2\n" +
		"PED-1;recibelo;Luis;Calle 456;Maipú;AO-1;1\n"
	res, err := s.ImportOrders(context.Background(), admin(), 7, "ordenes.csv", strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if res.OrdersCreated != 1 || res.TotalRows != 1 || res.FailedRows != 0 || len(orders.created) != 1 || orders.created[0] != "PED-1" {
		t.Fatalf("resultado: %+v; creadas: %v", res, orders.created)
	}
}

func TestImportOnlyExamplesHasNoData(t *testing.T) {
	orders := &importOrders{}
	s := templateService(orders)
	if _, err := s.ImportOrders(context.Background(), admin(), 7, "plantilla.xlsx", bytes.NewReader(template(t, s, admin(), 7))); err == nil {
		t.Fatal("una plantilla sin llenar no debe importarse")
	}
	if len(orders.created) != 0 {
		t.Fatalf("creó órdenes de ejemplo: %v", orders.created)
	}
}

package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/products/application"
	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
)

// fakeRepo: adaptador de prueba en memoria (sin PostgreSQL).
type fakeRepo struct{ items []domain.Product }

func (f *fakeRepo) List(_ context.Context, fl ports.ListFilter) ([]domain.Product, error) {
	var out []domain.Product
	for _, p := range f.items {
		if in(fl.CompanyIDs, p.CompanyID) {
			out = append(out, p)
		}
	}
	return out, nil
}
func (f *fakeRepo) GetByID(_ context.Context, cids []int64, id int64) (domain.Product, error) {
	for _, p := range f.items {
		if p.ID == id && in(cids, p.CompanyID) {
			return p, nil
		}
	}
	return domain.Product{}, domain.ErrNotFound
}
func (f *fakeRepo) Create(_ context.Context, p domain.Product) (domain.Product, error) {
	p.ID = int64(len(f.items) + 1)
	f.items = append(f.items, p)
	return p, nil
}

// in: nil = todas las empresas
func in(ids []int64, id int64) bool {
	if ids == nil {
		return true
	}
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestCreateNormalizesSKU(t *testing.T) {
	svc := application.NewService(&fakeRepo{})
	p, err := svc.Create(context.Background(), 1, application.CreateInput{SKU: " abc-1 ", Description: "Caja"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SKU != "ABC-1" || p.Type != domain.TypeSimple {
		t.Fatalf("inesperado: %+v", p)
	}
}

func TestCreateRejectsInvalid(t *testing.T) {
	svc := application.NewService(&fakeRepo{})
	_, err := svc.Create(context.Background(), 1, application.CreateInput{SKU: "", Description: "x"})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("se esperaba ErrInvalid, llegó %v", err)
	}
}

// Prueba negativa de aislamiento entre empresas (RF-03).
func TestCompanyIsolation(t *testing.T) {
	repo := &fakeRepo{}
	svc := application.NewService(repo)
	ctx := context.Background()
	p, _ := svc.Create(ctx, 1, application.CreateInput{SKU: "A", Description: "Empresa 1"})
	if _, err := svc.Get(ctx, []int64{2}, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("la empresa 2 no debe ver productos de la empresa 1")
	}
}

package application

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// Empresa del transporte y observaciones al cerrar el manifiesto (Traspaso Órdenes A3 y A4).
func TestCloseDetails(t *testing.T) {
	m := ports.Manifest{ID: 7, Lines: []ports.ManifestLine{{OrderID: 10, ExternalRef: "A-10"}, {OrderID: 11, ExternalRef: "A-11"}}}

	t.Run("campos propios", func(t *testing.T) {
		company, notes, obs, err := closeDetails(m, CloseInput{TransportCompany: "  Transportes Sur ", Notes: " sin novedad ",
			Observations: []ports.ManifestObservation{{OrderID: 10, Text: " faltó un cable "}, {OrderID: 11, Text: "  "}}})
		if err != nil {
			t.Fatal(err)
		}
		if company != "Transportes Sur" || notes != "sin novedad" {
			t.Fatalf("empresa %q, notas %q", company, notes)
		}
		if len(obs) != 1 || obs[0].OrderID != 10 || obs[0].Text != "faltó un cable" {
			t.Fatalf("observaciones %+v (la vacía se descarta)", obs)
		}
	})

	t.Run("front anterior: todo dentro de notes", func(t *testing.T) {
		notes := "Empresa del transporte: Transportes Sur\nretira con retraso\nObservación · #A-11 · caja abollada\nObservación · #Z-99 · no es del manifiesto"
		company, rest, obs, err := closeDetails(m, CloseInput{Notes: notes})
		if err != nil {
			t.Fatal(err)
		}
		if company != "Transportes Sur" {
			t.Fatalf("empresa %q", company)
		}
		if len(obs) != 1 || obs[0].OrderID != 11 || obs[0].Text != "caja abollada" {
			t.Fatalf("observaciones %+v", obs)
		}
		if rest != "retira con retraso\nObservación · #Z-99 · no es del manifiesto" {
			t.Fatalf("notas %q: solo se sacan las líneas reconocidas", rest)
		}
	})

	t.Run("rechazos", func(t *testing.T) {
		cases := []CloseInput{
			{Observations: []ports.ManifestObservation{{OrderID: 99, Text: "otra"}}},
			{Observations: []ports.ManifestObservation{{OrderID: 10, Text: strings.Repeat("x", maxObservation+1)}}},
			{TransportCompany: strings.Repeat("y", maxTransportCompany+1)},
		}
		for i, in := range cases {
			if _, _, _, err := closeDetails(m, in); !errors.Is(err, domain.ErrInvalid) {
				t.Errorf("caso %d: se esperaba ErrInvalid, llegó %v", i, err)
			}
		}
	})
}

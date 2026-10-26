package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

func TestFirstMatchingRuleWins(t *testing.T) {
	c1 := int64(1)
	rules := []ports.WaveRule{
		{Name: "pausada", Active: false, Courier: "starken", WaveCode: "X"},
		{Name: "chilexpress empresa 1", Active: true, Courier: "chilexpress", CompanyID: &c1, WaveCode: "AM"},
		{Name: "chilexpress", Active: true, Courier: "chilexpress", WaveCode: "PM"},
		{Name: "express", Active: true, DispatchType: "express", WaveCode: "GEN"},
	}
	o := domain.Order{CompanyID: 1, Courier: "chilexpress", DispatchType: "domicilio"}
	if r, ok := FirstMatch(rules, o); !ok || r.WaveCode != "AM" {
		t.Fatalf("debía ganar la primera coincidencia (AM): %+v", r)
	}
	o.CompanyID = 2
	if r, _ := FirstMatch(rules, o); r.WaveCode != "PM" {
		t.Fatalf("otra empresa -> PM, obtuve %s", r.WaveCode)
	}
	o = domain.Order{CompanyID: 2, Courier: "starken", DispatchType: "domicilio"}
	if _, ok := FirstMatch(rules, o); ok {
		t.Fatal("sin regla aplicable (la de starken está pausada): queda Validada sin ola")
	}
}

func TestWebhookSignature(t *testing.T) {
	body := []byte(`{"externalRef":"X"}`)
	mac := hmac.New(sha256.New, []byte("s3cr3t"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	if !VerifySignature("s3cr3t", body, sig, "") || !VerifySignature("s3cr3t", body, "sha256="+sig, "") {
		t.Fatal("firma válida rechazada")
	}
	if VerifySignature("s3cr3t", []byte(`{"externalRef":"Y"}`), sig, "") {
		t.Fatal("cuerpo alterado aceptado")
	}
	if VerifySignature("s3cr3t", body, "", "") || VerifySignature("s3cr3t", body, "", "otro") {
		t.Fatal("sin firma ni token válido debe rechazarse")
	}
	if !VerifySignature("s3cr3t", body, "", "s3cr3t") {
		t.Fatal("token correcto rechazado")
	}
}

func TestSecretBox(t *testing.T) {
	if _, err := NewSecretBox("corta"); err == nil {
		t.Fatal("clave inválida aceptada")
	}
	b, err := NewSecretBox("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := b.Seal("secreto")
	if enc == "secreto" {
		t.Fatal("no cifró")
	}
	if p, err := b.Open(enc); err != nil || p != "secreto" {
		t.Fatalf("no descifró: %v", err)
	}
	if _, err := (SecretBox{}).Seal("x"); err == nil {
		t.Fatal("sin clave no debe cifrar")
	}
}

func TestParseCSVWithSemicolonAndHeaders(t *testing.T) {
	data := []byte("\xef\xbb\xbfReferencia;Courier;Destinatario;Dirección;Comuna;SKU;Cantidad\nR1;starken;Ana;Calle 1;Ñuñoa;A;2\n")
	rows, err := parseTable("carga.csv", data)
	if err != nil || len(rows) != 2 {
		t.Fatalf("CSV con ; mal leído: %v %v", rows, err)
	}
	idx, missing := headerIndex(rows[0])
	if len(missing) != 0 {
		t.Fatalf("encabezados con mayúsculas/tildes deben reconocerse; faltan %v", missing)
	}
	if rows[1][idx["comuna"]] != "Ñuñoa" || rows[1][idx["cantidad"]] != "2" {
		t.Fatalf("columnas mal mapeadas: %v", idx)
	}
	if _, missing := headerIndex([]string{"referencia", "sku"}); len(missing) == 0 {
		t.Fatal("debe informar columnas faltantes")
	}
}

func TestDetectDocType(t *testing.T) {
	if detectDocType([]byte("%PDF-1.4 ..."), "x.exe") != "application/pdf" {
		t.Fatal("el tipo se decide por contenido, no por extensión")
	}
	if detectDocType([]byte("MZ\x90\x00binario"), "factura.pdf") == "application/pdf" {
		t.Fatal("un ejecutable renombrado a .pdf no es PDF")
	}
	if detectDocType([]byte(`<?xml version="1.0"?><DTE/>`), "f.xml") != "text/xml" {
		t.Fatal("XML de factura electrónica debe aceptarse")
	}
	if sanitizeFilename(`..\..\etc\passwd`, ".pdf") != "passwd" {
		t.Fatal("el nombre no debe conservar rutas")
	}
}

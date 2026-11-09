package domain

import (
	"strings"
	"testing"
)

func TestDraftEmailOrUsername(t *testing.T) {
	base := func() Draft {
		return Draft{Name: "Juan Pérez", Role: "empaque", AllCompanies: true}
	}
	cases := []struct {
		name     string
		mod      func(*Draft)
		ok       bool
		wantUser string
		msg      string
	}{
		{"con correo", func(d *Draft) { d.Email = " Juan@WeStorage.cl " }, true, "", ""},
		{"sin correo con usuario", func(d *Draft) { d.Username = " Bodega.Juan " }, true, "bodega.juan", ""},
		{"ni correo ni usuario", func(d *Draft) {}, false, "", "nombre de usuario"},
		{"ambos", func(d *Draft) { d.Email = "a@b.cl"; d.Username = "juan" }, false, "", "no ambos"},
		// Tarea 07: el perfil Cliente también puede crearse sin correo.
		{"cliente sin correo", func(d *Draft) {
			d.Username = "juan"
			d.Role = "cliente"
			d.AllCompanies = false
			d.Companies = []int64{1}
		}, true, "juan", ""},
		{"usuario con arroba", func(d *Draft) { d.Username = "juan@bodega" }, false, "", "nombre de usuario inválido"},
		{"usuario corto", func(d *Draft) { d.Username = "jp" }, false, "", "nombre de usuario inválido"},
		{"usuario empieza con punto", func(d *Draft) { d.Username = ".juan" }, false, "", "nombre de usuario inválido"},
		{"usuario con espacio", func(d *Draft) { d.Username = "juan perez" }, false, "", "nombre de usuario inválido"},
		{"jefe sin correo", func(d *Draft) { d.Username = "jefe_turno-2"; d.Role = "jefe_operaciones" }, true, "jefe_turno-2", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := base()
			c.mod(&d)
			msg, ok := d.Normalize()
			if ok != c.ok {
				t.Fatalf("ok=%v msg=%q", ok, msg)
			}
			if !ok && !strings.Contains(msg, c.msg) {
				t.Fatalf("mensaje %q no contiene %q", msg, c.msg)
			}
			if ok && d.Username != c.wantUser {
				t.Fatalf("usuario %q, quería %q", d.Username, c.wantUser)
			}
		})
	}
}

func TestDraftPermissions(t *testing.T) {
	cases := []struct {
		name  string
		draft Draft
		ok    bool
		msg   string
	}{
		{"admin con gestionar perfiles", Draft{Email: "a@b.cl", Name: "Ana", Role: "admin", Permissions: []string{" Gestionar_Perfiles "}, AllCompanies: true}, true, ""},
		{"permiso desconocido", Draft{Email: "a@b.cl", Name: "Ana", Role: "admin", Permissions: []string{"todo"}, AllCompanies: true}, false, "permiso desconocido"},
		{"cliente con gestionar perfiles", Draft{Email: "a@b.cl", Name: "Ana", Role: "cliente", Permissions: []string{"gestionar_perfiles"}, Companies: []int64{1}}, false, "permisos internos"},
		{"cliente con despacho", Draft{Email: "a@b.cl", Name: "Ana", Role: "cliente", Permissions: []string{"despacho"}, Companies: []int64{1}}, false, "permisos internos"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := c.draft
			msg, ok := d.Normalize()
			if ok != c.ok || (!ok && !strings.Contains(msg, c.msg)) {
				t.Fatalf("ok=%v msg=%q", ok, msg)
			}
			if ok && d.Permissions[0] != "gestionar_perfiles" {
				t.Fatalf("permiso normalizado %q", d.Permissions[0])
			}
		})
	}
}

func TestApplyKeepsUsername(t *testing.T) {
	name := "Otro Nombre"
	u := User{Username: "bodega.juan", Name: "Juan", Role: "empaque", AllCompanies: true}
	d := u.Apply(Changes{Name: &name})
	if msg, ok := d.Normalize(); !ok || d.Username != "bodega.juan" || d.Email != "" {
		t.Fatalf("editar un usuario sin correo debe conservar su usuario: %q %+v", msg, d)
	}
}

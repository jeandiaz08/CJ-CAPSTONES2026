package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/platform/xlsx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Profiles - perfiles de Clientes › Administración clientes (Tarea 07, scope «client») y de
// Configuración › Usuarios WeStorage (Tarea 08, scope «internal»). Reutiliza el Service de usuarios
// para lo que toca Keycloak (alta, nombre, rol, activar/desactivar, contraseña) y agrega el teléfono,
// el cargo, la empresa única del perfil Cliente, el rol y los permisos del perfil interno, la
// importación y la exportación. Quién puede hacer qué lo decide la ruta; aquí se comprueba que el
// perfil sea del alcance pedido y se protegen las reglas que no dependen de la ruta.
type Profiles struct {
	svc  *Service
	repo ports.ProfileRepository
}

func NewProfiles(svc *Service, repo ports.ProfileRepository) *Profiles {
	return &Profiles{svc: svc, repo: repo}
}

const (
	MaxImportRows  = 500
	MaxExportRows  = 10000
	importExample  = "EJEMPLO"
	profileSheet   = "Perfiles"
	instructionTab = "Instrucciones"
)

var (
	ErrScopeUnavailable = errors.New("alcance de perfiles inválido: usa client o internal")
	ErrExportTooBig     = errors.New("hay demasiados perfiles para exportar: acota los filtros")
	ErrReasonRequired   = ValidationError{"indica el motivo (entre 5 y 500 caracteres)"}
	// ErrLastManager (409): el cambio dejaría sin ningún Administrador activo con «Gestionar perfiles».
	ErrLastManager = errors.New("debe quedar al menos un Administrador activo con «Gestionar perfiles»: asígnale el permiso a otro Administrador antes de este cambio")
)

// ProfileView es el perfil que ve la pantalla. El Cliente tiene empresa y ningún permiso; el
// perfil interno tiene rol y permisos. TemporaryPassword (UserView) solo viene al crear un perfil
// interno sin correo.
type ProfileView struct {
	UserView
	Phone       string `json:"phone"`
	JobTitle    string `json:"jobTitle"`
	CompanyID   int64  `json:"companyId"`
	CompanyName string `json:"companyName"`
	CreatedAt   string `json:"createdAt"`
}

func profileView(p ports.Profile) ProfileView {
	return ProfileView{UserView: View(p.User), Phone: p.Phone, JobTitle: p.JobTitle, CompanyID: p.CompanyID, CompanyName: p.CompanyName,
		CreatedAt: p.CreatedAt.Format(time.RFC3339)}
}

func checkScope(scope domain.ProfileScope) error {
	if scope != domain.ProfileScopeClient && scope != domain.ProfileScopeInternal {
		return ErrScopeUnavailable
	}
	return nil
}

// inScope: el Cliente es de «client»; Administrador, Jefe de Operaciones y Empaque, de «internal».
func inScope(scope domain.ProfileScope, role string) bool {
	if scope == domain.ProfileScopeClient {
		return role == "cliente"
	}
	return slices.Contains(domain.ProfileRoles, role)
}

// isManager: Administrador activo con «Gestionar perfiles».
func isManager(u domain.User) bool {
	return u.Active && u.Role == "admin" && slices.Contains(u.Permissions, tenant.PermManageProfiles)
}

// keepsManager responde 409 si el perfil es hoy un Administrador activo con «Gestionar perfiles», el
// cambio le quita esa condición y no queda otro.
func (s *Profiles) keepsManager(ctx context.Context, cur domain.User, stillManager bool) error {
	if !isManager(cur) || stillManager {
		return nil
	}
	n, err := s.repo.CountManagers(ctx, cur.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLastManager
	}
	return nil
}

func (s *Profiles) List(ctx context.Context, f ports.ProfileFilter) ([]ProfileView, int, error) {
	if err := checkScope(f.Scope); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 20
	}
	f.Offset = max(f.Offset, 0)
	list, total, err := s.repo.ListProfiles(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ProfileView, 0, len(list))
	for _, p := range list {
		out = append(out, profileView(p))
	}
	return out, total, nil
}

// get devuelve el perfil solo si es del alcance (un perfil interno no existe para «client» y
// viceversa).
func (s *Profiles) get(ctx context.Context, scope domain.ProfileScope, id int64) (ports.Profile, error) {
	if err := checkScope(scope); err != nil {
		return ports.Profile{}, err
	}
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return p, err
	}
	if !inScope(scope, p.Role) {
		return ports.Profile{}, domain.ErrNotFound
	}
	return p, nil
}

func (s *Profiles) Get(ctx context.Context, scope domain.ProfileScope, id int64) (ProfileView, error) {
	p, err := s.get(ctx, scope, id)
	return profileView(p), err
}

// companyErrors valida que la empresa exista y esté activa (un perfil nuevo no se crea en una
// empresa desactivada).
func (s *Profiles) companyErrors(ctx context.Context, id int64) domain.ValidationErrors {
	if id <= 0 {
		return nil
	}
	_, active, err := s.repo.Company(ctx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return domain.ValidationErrors{{Field: "companyId", Message: "La empresa no existe."}}
	case err != nil:
		return nil // la valida después el alta
	case !active:
		return domain.ValidationErrors{{Field: "companyId", Message: "La empresa está desactivada: actívala antes de agregarle perfiles."}}
	}
	return nil
}

// Create da de alta un perfil. Cliente: rol fijo, sin permisos, una empresa y siempre con correo
// (Keycloak le envía el enlace para definir la contraseña). Interno: rol y permisos del formulario,
// todas las empresas, y con correo o con nombre de usuario; sin correo, la respuesta trae una sola
// vez la contraseña temporal.
func (s *Profiles) Create(ctx context.Context, actor tenant.Identity, scope domain.ProfileScope, d domain.ProfileDraft) (ProfileView, error) {
	if err := checkScope(scope); err != nil {
		return ProfileView{}, err
	}
	errs := d.Normalize(scope, true)
	draft := domain.Draft{Email: d.Email, Username: d.Username, Name: d.Name, Role: d.Role, Permissions: d.Permissions,
		AllCompanies: true, Companies: []int64{}}
	if scope == domain.ProfileScopeClient {
		errs = append(errs, s.companyErrors(ctx, d.CompanyID)...)
		draft = domain.Draft{Email: d.Email, Name: d.Name, Role: "cliente", Permissions: []string{}, Companies: []int64{d.CompanyID}}
	}
	if len(errs) > 0 {
		return ProfileView{}, errs
	}
	u, err := s.svc.Create(ctx, actor, draft)
	if err != nil {
		return ProfileView{}, loginError(err)
	}
	if d.Phone != "" || d.JobTitle != "" {
		// Teléfono y cargo son parte del alta: el registro ya tiene la acción «created».
		if err := s.repo.SetContact(ctx, u.ID, d.Phone, d.JobTitle); err != nil {
			return ProfileView{}, err
		}
	}
	p, err := s.repo.GetProfile(ctx, u.ID)
	if err != nil {
		return ProfileView{}, err
	}
	v := profileView(p)
	v.TemporaryPassword = u.TemporaryPassword
	return v, nil
}

// loginError lleva al campo el correo o el nombre de usuario repetido.
func loginError(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmailTaken):
		return conflictErr{domain.ValidationErrors{{Field: "email", Message: "Ese correo ya tiene un perfil."}}}
	case errors.Is(err, domain.ErrUsernameTaken):
		return conflictErr{domain.ValidationErrors{{Field: "username", Message: "Ese nombre de usuario ya existe."}}}
	}
	return err
}

// conflictErr: correo o usuario repetido (409 con el error en el campo).
type conflictErr struct{ domain.ValidationErrors }

func (c conflictErr) Unwrap() error { return c.ValidationErrors }

// IsConflict indica si el error es un 409 por correo o usuario repetido.
func IsConflict(err error) bool {
	var c conflictErr
	return errors.As(err, &c)
}

func sortedCopy(v []string) []string {
	out := slices.Clone(v)
	slices.Sort(out)
	return out
}

// Update edita nombre, teléfono y cargo, más la empresa (Cliente) o el rol y los permisos
// (interno). El correo y el usuario no cambian. Nadie cambia su propio rol, permisos ni empresa.
// El nombre y el rol se cambian primero en Keycloak; si algo falla después, se deshace. Deja una
// sola acción en el registro con el antes y el después.
func (s *Profiles) Update(ctx context.Context, actor tenant.Identity, scope domain.ProfileScope, id int64, d domain.ProfileDraft) (ProfileView, error) {
	cur, err := s.get(ctx, scope, id)
	if err != nil {
		return ProfileView{}, err
	}
	internal := scope == domain.ProfileScopeInternal
	errs := d.Normalize(scope, false)
	if !internal && d.CompanyID != cur.CompanyID {
		errs = append(errs, s.companyErrors(ctx, d.CompanyID)...)
	}
	if len(errs) > 0 {
		return ProfileView{}, errs
	}
	name := d.Name
	changes := domain.Changes{Name: &name}
	if internal {
		if id == actor.UserID && (d.Role != cur.Role || !slices.Equal(d.Permissions, sortedCopy(cur.Permissions))) {
			return ProfileView{}, domain.ErrSelfChange
		}
		changes.Role, changes.Permissions = &d.Role, &d.Permissions
	} else {
		if id == actor.UserID && d.CompanyID != cur.CompanyID {
			return ProfileView{}, domain.ErrSelfChange
		}
		companies := []int64{d.CompanyID}
		changes.Companies = &companies
	}
	draft := cur.Apply(changes)
	if err := s.svc.validate(ctx, &draft); err != nil {
		return ProfileView{}, err
	}
	diff := map[string]any{}
	change := func(key string, from, to any) {
		if fmt.Sprint(from) != fmt.Sprint(to) {
			diff[key] = map[string]any{"from": from, "to": to}
		}
	}
	change("name", cur.Name, d.Name)
	change("phone", cur.Phone, d.Phone)
	change("jobTitle", cur.JobTitle, d.JobTitle)
	if internal {
		change("role", cur.Role, d.Role)
		change("permissions", sortedCopy(cur.Permissions), d.Permissions)
		if err := s.keepsManager(ctx, cur.User, d.Role == "admin" && slices.Contains(d.Permissions, tenant.PermManageProfiles)); err != nil {
			return ProfileView{}, err
		}
	} else if !slices.Equal(cur.Companies, draft.Companies) {
		newName, _, _ := s.repo.Company(ctx, d.CompanyID)
		diff["companyName"] = map[string]any{"from": cur.CompanyName, "to": newName}
	}
	if len(diff) == 0 {
		return profileView(cur), nil
	}

	// Cambios con vuelta atrás: si un paso falla, se deshacen los anteriores (Keycloak y Core).
	var undo []func(context.Context) error
	rollback := func() {
		bg := context.WithoutCancel(ctx)
		for i := len(undo) - 1; i >= 0; i-- {
			if e := undo[i](bg); e != nil {
				s.svc.log.Error("no se pudo deshacer un paso de la edición del perfil", "user", id, "err", e)
			}
		}
	}
	if cur.KeycloakID != "" && d.Name != cur.Name {
		if err := s.svc.idp.UpdateName(ctx, cur.KeycloakID, d.Name); err != nil {
			return ProfileView{}, err
		}
		undo = append(undo, func(c context.Context) error { return s.svc.idp.UpdateName(c, cur.KeycloakID, cur.Name) })
	}
	if cur.KeycloakID != "" && draft.Role != cur.Role {
		if err := s.svc.idp.SetRole(ctx, cur.KeycloakID, draft.Role); err != nil {
			rollback()
			return ProfileView{}, err
		}
		undo = append(undo, func(c context.Context) error { return s.svc.idp.SetRole(c, cur.KeycloakID, cur.Role) })
	}
	if err := s.svc.users.Update(ctx, id, draft, actor.UserID); err != nil {
		rollback()
		return ProfileView{}, err
	}
	before := cur.Apply(domain.Changes{})
	undo = append(undo, func(c context.Context) error { return s.svc.users.Update(c, id, before, actor.UserID) })
	if err := s.repo.SetContact(ctx, id, d.Phone, d.JobTitle); err != nil {
		rollback()
		return ProfileView{}, err
	}
	_ = s.svc.users.Audit(ctx, &actor.UserID, id, "updated", diff)
	return s.Get(ctx, scope, id)
}

// SetActive activa o desactiva el perfil (Keycloak lo bloquea y cierra sus sesiones al
// desactivarlo). Desactivar exige motivo. Nadie se desactiva a sí mismo, y no se desactiva al
// último Administrador activo con «Gestionar perfiles».
func (s *Profiles) SetActive(ctx context.Context, actor tenant.Identity, scope domain.ProfileScope, id int64, active bool, reason string) (ProfileView, error) {
	cur, err := s.get(ctx, scope, id)
	if err != nil {
		return ProfileView{}, err
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); (!active && n < 5) || n > 500 {
		return ProfileView{}, ErrReasonRequired
	}
	if cur.Active == active {
		return ProfileView{}, ErrSameState
	}
	if id == actor.UserID {
		return ProfileView{}, domain.ErrSelfChange
	}
	if !active {
		if err := s.keepsManager(ctx, cur.User, false); err != nil {
			return ProfileView{}, err
		}
	}
	if active {
		_, err = s.svc.Enable(ctx, actor, id, reason)
	} else {
		_, err = s.svc.Disable(ctx, actor, id, reason)
	}
	if err != nil {
		return ProfileView{}, err
	}
	return s.Get(ctx, scope, id)
}

// ErrSameState: el perfil ya está en ese estado.
var ErrSameState = errors.New("el perfil ya está en ese estado; recarga la página")

// ResetPassword: con correo, Keycloak le envía el enlace para definir una contraseña nueva. Un perfil
// interno sin correo recibe una contraseña temporal que se devuelve una sola vez (SCRUM-231). Un
// Cliente sin correo (datos antiguos) no se puede restablecer aquí.
func (s *Profiles) ResetPassword(ctx context.Context, actor tenant.Identity, scope domain.ProfileScope, id int64) (string, error) {
	cur, err := s.get(ctx, scope, id)
	if err != nil {
		return "", err
	}
	if !cur.HasEmail() {
		if scope == domain.ProfileScopeInternal {
			return s.svc.ResetPassword(ctx, actor, id)
		}
		return "", ValidationError{"el perfil no tiene correo; edítalo para regularizarlo antes de restablecer la contraseña"}
	}
	return "", s.svc.ResendSetPassword(ctx, actor, id)
}

// ---------------------------------------------------------------------------
// Excel: exportar, plantilla e importación masiva
// ---------------------------------------------------------------------------

func stateText(active bool) string {
	if active {
		return "Activo"
	}
	return "Inactivo"
}

// Nombres en pantalla y en Excel de roles y permisos internos.
var (
	roleLabels = map[string]string{"admin": "Administrador", "jefe_operaciones": "Jefe de Operaciones", "empaque": "Empaque", "cliente": "Cliente"}
	permLabels = map[string]string{"despacho": "Despacho", "gestionar_perfiles": "Gestionar perfiles"}
)

func permsText(perms []string) string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if l, ok := permLabels[p]; ok {
			out = append(out, l)
		}
	}
	return strings.Join(out, ", ")
}

func (s *Profiles) Export(ctx context.Context, f ports.ProfileFilter, w io.Writer) error {
	if err := checkScope(f.Scope); err != nil {
		return err
	}
	f.Limit, f.Offset = MaxExportRows+1, 0
	list, _, err := s.repo.ListProfiles(ctx, f)
	if err != nil {
		return err
	}
	if len(list) > MaxExportRows {
		return ErrExportTooBig
	}
	internal := f.Scope == domain.ProfileScopeInternal
	rows := make([][]any, 0, len(list))
	for _, p := range list {
		last := ""
		if p.LastLoginAt != nil {
			last = p.LastLoginAt.In(chile).Format("02-01-2006 15:04")
		}
		created := p.CreatedAt.In(chile).Format("02-01-2006 15:04")
		if internal {
			login := p.Email
			if login == "" {
				login = p.Username
			}
			rows = append(rows, []any{p.Name, login, roleLabels[p.Role], permsText(p.Permissions), p.Phone, p.JobTitle, stateText(p.Active), created, last})
			continue
		}
		rows = append(rows, []any{p.Name, p.CompanyName, p.Email, p.Phone, p.JobTitle, "Cliente", stateText(p.Active), created, last})
	}
	if internal {
		// TODO(PV-21): columna «Bodegas» cuando exista la tabla de bodegas del usuario.
		return xlsx.Write(w, profileSheet, []string{"Nombre completo", "Correo o usuario", "Rol", "Permisos", "Teléfono", "Cargo", "Estado",
			"Fecha de creación", "Último acceso"}, rows)
	}
	return xlsx.Write(w, profileSheet, []string{"Nombre perfil", "Nombre empresa", "Correo", "Teléfono", "Cargo", "Rol", "Estado",
		"Fecha de creación", "Último acceso"}, rows)
}

var (
	importHeader         = []string{"Nombre completo", "RUT empresa", "Correo", "Teléfono", "Cargo"}
	internalImportHeader = []string{"Nombre completo", "Correo", "Rol", "Permisos", "Teléfono", "Cargo"}
)

// Template: hoja «Perfiles» con el encabezado y una fila de ejemplo (no se importa), y hoja
// «Instrucciones» con las reglas (y, para clientes, las empresas activas).
func (s *Profiles) Template(ctx context.Context, scope domain.ProfileScope, w io.Writer) error {
	if err := checkScope(scope); err != nil {
		return err
	}
	if scope == domain.ProfileScopeInternal {
		rows := [][]any{{importExample + " Ana Torres", "ana.torres@westorage.cl", "Jefe de Operaciones", "Despacho", "+56 9 1234 5678", "Supervisora"}}
		info := [][]any{
			{"Una fila por perfil de WeStorage.", ""},
			{"Obligatorios: Nombre completo, Correo y Rol. Cada perfil recibe en su correo el enlace para definir su contraseña.", ""},
			{"El personal sin correo se crea uno a uno con «Nuevo perfil» (se le entrega una contraseña temporal).", ""},
			{"Rol: Administrador, Jefe de Operaciones o Empaque.", ""},
			{"Permisos (opcional, separados por coma): Despacho (Jefe de Operaciones y Empaque) o Gestionar perfiles (solo Administrador).", ""},
			{"Las filas cuyo nombre empieza con «EJEMPLO» no se importan.", ""},
			{fmt.Sprintf("Máximo %d filas por archivo. Las filas válidas se crean; las inválidas se informan con su fila y motivo.", MaxImportRows), ""},
		}
		return xlsx.WriteWorkbook(w, []xlsx.Sheet{
			{Name: profileSheet, Header: internalImportHeader, Rows: rows, Widths: []int{28, 30, 22, 24, 18, 24}},
			{Name: instructionTab, Header: []string{"Instrucciones", ""}, Rows: info, Widths: []int{110, 4}},
		})
	}
	companies, err := s.repo.ActiveCompanies(ctx)
	if err != nil {
		return err
	}
	rows := [][]any{{importExample + " Juan Pérez", "76.123.456-0", "juan.perez@empresa.cl", "+56 9 1234 5678", "Encargado de bodega"}}
	info := [][]any{
		{"Una fila por perfil. El rol es siempre Cliente, sin permisos internos.", ""},
		{"Obligatorios: Nombre completo, RUT empresa (de una empresa activa) y Correo.", ""},
		{"Cada perfil recibe en su correo el enlace para definir su contraseña.", ""},
		{"Las filas cuyo nombre empieza con «EJEMPLO» no se importan.", ""},
		{fmt.Sprintf("Máximo %d filas por archivo. Las filas válidas se crean; las inválidas se informan con su fila y motivo.", MaxImportRows), ""},
		{"", ""},
		{"Empresas activas", "RUT"},
	}
	for _, c := range companies {
		info = append(info, []any{c[0], c[1]})
	}
	return xlsx.WriteWorkbook(w, []xlsx.Sheet{
		{Name: profileSheet, Header: importHeader, Rows: rows, Widths: []int{28, 16, 30, 18, 24}},
		{Name: instructionTab, Header: []string{"Instrucciones", ""}, Rows: info, Widths: []int{90, 16}},
	})
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ImportedProfile struct {
	Row   int    `json:"row"`
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ImportResult struct {
	Rows    int               `json:"rows"`
	Created []ImportedProfile `json:"created"`
	Errors  []ImportRowError  `json:"errors"`
}

// key normaliza un texto del Excel para compararlo: minúsculas, sin tildes, «_» como espacio.
func key(s string) string {
	s = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "_", " ", ".", "").Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

var (
	roleByText = map[string]string{"administrador": "admin", "admin": "admin", "jefe de operaciones": "jefe_operaciones",
		"jefe operaciones": "jefe_operaciones", "empaque": "empaque"}
	permByText = map[string]string{"despacho": "despacho", "gestionar perfiles": "gestionar_perfiles", "adm perfiles": "gestionar_perfiles"}
)

// Import crea las filas válidas e informa las inválidas con su fila y motivo, como la carga de
// órdenes. Cada perfil creado queda en el registro y, en clientes, por cada empresa una acción
// «Carga masiva».
func (s *Profiles) Import(ctx context.Context, actor tenant.Identity, scope domain.ProfileScope, data []byte) (ImportResult, error) {
	res := ImportResult{Created: []ImportedProfile{}, Errors: []ImportRowError{}}
	if err := checkScope(scope); err != nil {
		return res, err
	}
	internal := scope == domain.ProfileScopeInternal
	header, required := importHeader, importHeader[:2]
	if internal {
		header, required = internalImportHeader, []string{internalImportHeader[0], internalImportHeader[2]}
	}
	table, err := xlsx.ReadFirstSheet(data)
	if err != nil || len(table) == 0 {
		return res, ValidationError{"no se pudo leer el archivo: usa la plantilla .xlsx de perfiles"}
	}
	col := map[string]int{}
	for i, h := range table[0] {
		col[strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(h), "*")))] = i
	}
	for _, h := range required {
		if _, ok := col[strings.ToLower(h)]; !ok {
			return res, ValidationError{fmt.Sprintf("falta la columna «%s»: usa la plantilla de perfiles", h)}
		}
	}
	cell := func(r []string, name string) string {
		if i, ok := col[strings.ToLower(name)]; ok && i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	if len(table)-1 > MaxImportRows {
		return res, ValidationError{fmt.Sprintf("el archivo tiene %d filas; el máximo es %d", len(table)-1, MaxImportRows)}
	}
	seen := map[string]int{}
	perCompany := map[int64]int{}
	for i, r := range table[1:] {
		row := i + 2 // fila de Excel (la 1 es el encabezado)
		name := cell(r, header[0])
		if strings.Join(r, "") == "" || strings.HasPrefix(strings.ToUpper(name), importExample) {
			continue
		}
		res.Rows++
		fail := func(field, msg string) {
			res.Errors = append(res.Errors, ImportRowError{Row: row, Field: field, Message: msg})
		}
		var d domain.ProfileDraft
		if internal {
			d = domain.ProfileDraft{Name: name, Email: cell(r, "Correo"), Phone: cell(r, "Teléfono"), JobTitle: cell(r, "Cargo")}
			if d.Email == "" {
				fail("email", "Falta el correo: el personal sin correo se crea uno a uno con «Nuevo perfil».")
				continue
			}
			role, ok := roleByText[key(cell(r, "Rol"))]
			if !ok {
				fail("role", fmt.Sprintf("Rol «%s» no válido: usa Administrador, Jefe de Operaciones o Empaque.", cell(r, "Rol")))
				continue
			}
			d.Role = role
			bad := ""
			for _, p := range strings.FieldsFunc(cell(r, "Permisos"), func(c rune) bool { return c == ',' || c == ';' }) {
				if strings.TrimSpace(p) == "" {
					continue
				}
				perm, ok := permByText[key(p)]
				if !ok {
					bad = strings.TrimSpace(p)
					break
				}
				d.Permissions = append(d.Permissions, perm)
			}
			if bad != "" {
				fail("permissions", fmt.Sprintf("Permiso «%s» no válido: usa Despacho o Gestionar perfiles.", bad))
				continue
			}
		} else {
			d = domain.ProfileDraft{Name: name, Email: cell(r, importHeader[2]), Phone: cell(r, importHeader[3]), JobTitle: cell(r, importHeader[4])}
			rut := cell(r, importHeader[1])
			if rut == "" {
				fail("companyId", "Falta el RUT de la empresa.")
				continue
			}
			cid, _, active, err := s.repo.CompanyByTaxID(ctx, rut)
			switch {
			case errors.Is(err, domain.ErrNotFound):
				fail("companyId", fmt.Sprintf("No hay una empresa con el RUT %s.", rut))
				continue
			case err != nil:
				return res, err
			case !active:
				fail("companyId", fmt.Sprintf("La empresa con RUT %s está desactivada.", rut))
				continue
			}
			d.CompanyID = cid
		}
		if errs := d.Normalize(scope, true); len(errs) > 0 {
			fail(errs[0].Field, errs[0].Message)
			continue
		}
		if prev, dup := seen[d.Email]; dup {
			fail("email", fmt.Sprintf("Correo repetido en el archivo (fila %d).", prev))
			continue
		}
		seen[d.Email] = row
		v, err := s.Create(ctx, actor, scope, d)
		if err != nil {
			var ve domain.ValidationErrors
			var ae ValidationError
			switch {
			case errors.As(err, &ve):
				fail(ve[0].Field, ve[0].Message)
			case errors.As(err, &ae):
				fail("", ae.Msg)
			default:
				return res, err // sin Keycloak (o ante un error inesperado) no tiene sentido seguir
			}
			continue
		}
		res.Created = append(res.Created, ImportedProfile{Row: row, ID: v.ID, Name: v.Name, Email: v.Email})
		if !internal {
			perCompany[d.CompanyID]++
		}
	}
	for cid, n := range perCompany {
		_ = s.repo.AuditCompany(ctx, actor.UserID, cid, "profiles_imported", map[string]any{"created": n, "rows": res.Rows, "failed": len(res.Errors)})
	}
	return res, nil
}

// chile: fechas del Excel en hora de Chile (si la imagen no trae tzdata, UTC-3).
var chile = func() *time.Location {
	if loc, err := time.LoadLocation("America/Santiago"); err == nil {
		return loc
	}
	return time.FixedZone("CLT", -3*3600)
}()

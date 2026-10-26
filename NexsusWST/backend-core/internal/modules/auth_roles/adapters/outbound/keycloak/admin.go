// Package keycloak implementa ports.IdentityProvider con la Admin API de
// Keycloak, usando la cuenta de servicio del cliente "nexus-core"
// (roles manage-users, view-users, query-users y view-realm).
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

type Config struct {
	BaseURL      string // URL interna, p. ej. http://keycloak:8080/auth
	Realm        string // nexuswst
	ClientID     string // nexus-core
	ClientSecret string
	AppURL       string // a dónde vuelve el usuario después de definir su contraseña
	WebClientID  string // nexus-web
	EmailLinkTTL time.Duration
}

type Admin struct {
	cfg    Config
	http   *http.Client
	mu     sync.Mutex
	token  string
	expiry time.Time
}

func New(cfg Config) *Admin {
	if cfg.EmailLinkTTL == 0 {
		cfg.EmailLinkTTL = 24 * time.Hour
	}
	return &Admin{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}}
}

// baseRoles son los roles de Keycloak que representan el perfil de Nexus.
var baseRoles = map[string]bool{"admin": true, "jefe_operaciones": true, "analista": true, "empaque": true, "cliente": true}

// ---- token de la cuenta de servicio --------------------------------------

func (a *Admin) accessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Until(a.expiry) > 30*time.Second {
		return a.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {a.cfg.ClientID}, "client_secret": {a.cfg.ClientSecret}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.BaseURL+"/realms/"+a.cfg.Realm+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrIdPUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("%w: token de servicio HTTP %d %s", domain.ErrIdPUnavailable, res.StatusCode, b)
	}
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return "", err
	}
	a.token, a.expiry = t.AccessToken, time.Now().Add(time.Duration(t.ExpiresIn)*time.Second)
	return a.token, nil
}

// do llama a /admin/realms/{realm}{path}.
func (a *Admin) do(ctx context.Context, method, path string, query url.Values, body any, out any) (*http.Response, error) {
	tok, err := a.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	u := a.cfg.BaseURL + "/admin/realms/" + a.cfg.Realm + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, method, u, rd)
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrIdPUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		a.mu.Lock()
		a.token = "" // se renovará en la próxima llamada
		a.mu.Unlock()
	}
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return res, &HTTPError{Status: res.StatusCode, Body: string(b), Op: method + " " + path}
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return res, err
		}
	}
	return res, nil
}

type HTTPError struct {
	Status int
	Body   string
	Op     string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("keycloak %s: HTTP %d %s", e.Op, e.Status, e.Body)
}

func status(err error) int {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status
	}
	return 0
}

// ---- operaciones ------------------------------------------------------------

func splitName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, " "); i > 0 {
		return name[:i], strings.TrimSpace(name[i+1:])
	}
	return name, ""
}

func (a *Admin) CreateUser(ctx context.Context, email, name string) (string, error) {
	first, last := splitName(name)
	res, err := a.do(ctx, http.MethodPost, "/users", nil, map[string]any{
		"username": email, "email": email, "firstName": first, "lastName": last,
		"enabled": true, "emailVerified": true, // el enlace de un solo uso llega a ese correo
	}, nil)
	if status(err) == http.StatusConflict {
		return "", domain.ErrEmailTaken
	}
	if err != nil {
		return "", err
	}
	// Location: .../users/{id}
	loc := res.Header.Get("Location")
	if i := strings.LastIndex(loc, "/"); i >= 0 && i < len(loc)-1 {
		return loc[i+1:], nil
	}
	return a.FindByEmail(ctx, email)
}

func (a *Admin) FindByEmail(ctx context.Context, email string) (string, error) {
	var users []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if _, err := a.do(ctx, http.MethodGet, "/users", url.Values{"email": {email}, "exact": {"true"}}, nil, &users); err != nil {
		return "", err
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u.ID, nil
		}
	}
	return "", domain.ErrNotFound
}

func (a *Admin) Exists(ctx context.Context, id string) (bool, error) {
	_, err := a.do(ctx, http.MethodGet, "/users/"+url.PathEscape(id), nil, nil, nil)
	if status(err) == http.StatusNotFound {
		return false, nil
	}
	return err == nil, err
}

func (a *Admin) UpdateName(ctx context.Context, id, name string) error {
	first, last := splitName(name)
	_, err := a.do(ctx, http.MethodPut, "/users/"+url.PathEscape(id), nil, map[string]any{"firstName": first, "lastName": last}, nil)
	return err
}

type roleRep struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SetRole deja exactamente un rol base (los demás roles de Keycloak no se tocan).
func (a *Admin) SetRole(ctx context.Context, id, role string) error {
	if !baseRoles[role] {
		return domain.ErrInvalid
	}
	base := "/users/" + url.PathEscape(id) + "/role-mappings/realm"
	var current []roleRep
	if _, err := a.do(ctx, http.MethodGet, base, nil, nil, &current); err != nil {
		return err
	}
	var remove []roleRep
	has := false
	for _, r := range current {
		if r.Name == role {
			has = true
		} else if baseRoles[r.Name] {
			remove = append(remove, r)
		}
	}
	if len(remove) > 0 {
		if _, err := a.do(ctx, http.MethodDelete, base, nil, remove, nil); err != nil {
			return err
		}
	}
	if has {
		return nil
	}
	var target roleRep
	if _, err := a.do(ctx, http.MethodGet, "/roles/"+url.PathEscape(role), nil, nil, &target); err != nil {
		return err
	}
	_, err := a.do(ctx, http.MethodPost, base, nil, []roleRep{target}, nil)
	return err
}

func (a *Admin) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := a.do(ctx, http.MethodPut, "/users/"+url.PathEscape(id), nil, map[string]any{"enabled": enabled}, nil)
	return err
}

func (a *Admin) Logout(ctx context.Context, id string) error {
	_, err := a.do(ctx, http.MethodPost, "/users/"+url.PathEscape(id)+"/logout", nil, nil, nil)
	return err
}

// SendSetPasswordEmail envía el correo con un enlace de un solo uso para
// definir la contraseña (lo envía Keycloak con el SMTP de westorage.cl).
func (a *Admin) SendSetPasswordEmail(ctx context.Context, id string) error {
	q := url.Values{
		"lifespan":     {fmt.Sprint(int(a.cfg.EmailLinkTTL.Seconds()))},
		"client_id":    {a.cfg.WebClientID},
		"redirect_uri": {strings.TrimRight(a.cfg.AppURL, "/") + "/"},
	}
	_, err := a.do(ctx, http.MethodPut, "/users/"+url.PathEscape(id)+"/execute-actions-email", q, []string{"UPDATE_PASSWORD"}, nil)
	return err
}

func (a *Admin) DeleteUser(ctx context.Context, id string) error {
	_, err := a.do(ctx, http.MethodDelete, "/users/"+url.PathEscape(id), nil, nil, nil)
	if status(err) == http.StatusNotFound {
		return nil
	}
	return err
}

package tenant

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGProfiles lee el perfil desde PostgreSQL (tablas del Core). Órdenes usa
// esta misma lectura en solo lectura (DA-12): no depende del Core.
type PGProfiles struct{ DB *pgxpool.Pool }

const profileSQL = `
SELECT u.id, COALESCE(u.keycloak_id::text, ''), u.email, u.name, u.role, u.permissions,
       u.all_companies, u.active,
       COALESCE(array_agg(c.id ORDER BY c.id) FILTER (WHERE c.active), '{}'),
       COUNT(c.id) FILTER (WHERE NOT c.active)
FROM users u
LEFT JOIN user_companies uc ON uc.user_id = u.id
LEFT JOIN companies c       ON c.id = uc.company_id
WHERE u.keycloak_id = $1
GROUP BY u.id`

func (s PGProfiles) ProfileByKeycloakID(ctx context.Context, keycloakID string) (Profile, error) {
	var p Profile
	err := s.DB.QueryRow(ctx, profileSQL, keycloakID).Scan(&p.UserID, &p.KeycloakID, &p.Email, &p.Name,
		&p.Role, &p.Permissions, &p.AllCompanies, &p.Active, &p.Companies, &p.DisabledCompanies)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	if p.Permissions == nil {
		p.Permissions = []string{}
	}
	return p, err
}

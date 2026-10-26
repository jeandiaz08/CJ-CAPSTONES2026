-- ============================================================================
-- DATOS DE PRUEBA v1.1 (ficticios). ELIMINAR antes de producción.
-- Los usuarios de prueba existen en Keycloak (infrastructure/keycloak/dev-users)
-- con la contraseña Nexus2026!. Aquí solo va su PERFIL de Nexus; el vínculo
-- con Keycloak (keycloak_id) se completa solo en su primer ingreso, por correo.
-- ============================================================================
INSERT INTO users (email, name, role, permissions, all_companies)
VALUES ('cliente2@nexus.local', 'Cliente Demo Dos', 'cliente', '{}', FALSE)
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_companies (user_id, company_id)
SELECT id, 2 FROM users WHERE email = 'cliente2@nexus.local'
ON CONFLICT DO NOTHING;

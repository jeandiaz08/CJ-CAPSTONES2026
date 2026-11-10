// Configuración › Usuarios WeStorage (Tarea 08). Fase 1: perfiles internos (scope=internal).
// Usa el seed: admin@nexus.local con «Gestionar perfiles», admin.lectura@nexus.local sin él, y los
// perfiles internos del seed de demostración (Esperanza Gómez, Pedro Bodega sin correo, Benjamín
// Cueto inactivo). Lo que crean estas pruebas usa @e2e.westorage.cl o el usuario «e2e.…».
import { expect, test, type Page } from "@playwright/test";
import { coreApi, login, mailSubjects, ordersApi } from "./helpers";

const sidebar = (page: Page) => page.locator(".sidebar");
type Profile = { id: number; name: string; role: string; permissions: string[]; username?: string; temporaryPassword?: string };
type Req = { id: number; scope: string; status: string; companyId: number; temporaryPassword?: string; profile: { name: string; username?: string; role?: string } };
type Activity = { items: { action: string; entity: { type: string; id: string }; detailLines: string[] }[] };

test.describe("Usuarios WeStorage · Fase 1: API", () => {
  test("consulta solo el Administrador; cambios con el permiso; rol en Keycloak y Core, con registro", async () => {
    const admin = await coreApi("admin@nexus.local");
    const reader = await coreApi("admin.lectura@nexus.local");
    const jefe = await coreApi("jefe@nexus.local");
    const cliente = await coreApi("cliente1@nexus.local");

    const list = await reader<{ data: Profile[] }>("GET", "/v1/profiles?scope=internal&limit=200");
    expect(list.status).toBe(200);
    expect(list.body.data.some((p) => p.role === "cliente")).toBe(false);
    expect(list.body.data.some((p) => p.role === "analista")).toBe(false); // perfil futuro: no se lista
    expect((await jefe("GET", "/v1/profiles?scope=internal")).status).toBe(403);
    expect((await cliente("GET", "/v1/profiles?scope=internal")).status).toBe(403);
    expect((await reader("POST", "/v1/profiles?scope=internal", { name: "X", email: "x@e2e.westorage.cl", role: "empaque" })).status).toBe(403);

    // Rol no ofrecido y permiso que no corresponde al rol: 422 en el campo.
    expect(JSON.stringify((await admin("POST", "/v1/profiles?scope=internal", { name: "X", email: "x@e2e.westorage.cl", role: "analista" })).body)).toContain('"field":"role"');
    expect(JSON.stringify((await admin("POST", "/v1/profiles?scope=internal", { name: "X", email: "x@e2e.westorage.cl", role: "empaque", permissions: ["gestionar_perfiles"] })).body)).toContain('"field":"permissions"');

    // Sin correo: usuario obligatorio y contraseña temporal una sola vez.
    const stamp = Date.now();
    const created = await admin<Profile>("POST", "/v1/profiles?scope=internal", { name: `E2E Interno ${stamp}`, username: `e2e.int.${stamp}`, role: "empaque", permissions: ["despacho"] });
    expect(created.status).toBe(201);
    expect(created.body.temporaryPassword).toBeTruthy();
    const id = created.body.id;
    expect((await admin<Profile>("GET", `/v1/profiles/${id}?scope=internal`)).body.temporaryPassword).toBeUndefined();

    // Cambio de rol: queda en el Core (y en Keycloak; si fallara, se deshace) y en el registro.
    const updated = await admin<Profile>("PATCH", `/v1/profiles/${id}?scope=internal`, { name: `E2E Interno ${stamp}`, role: "jefe_operaciones", permissions: ["despacho"] });
    expect(updated.status).toBe(200);
    expect(updated.body.role).toBe("jefe_operaciones");
    const reset = await admin<{ temporaryPassword?: string }>("POST", `/v1/profiles/${id}/password?scope=internal`);
    expect(reset.status).toBe(200);
    expect(reset.body.temporaryPassword).toBeTruthy();

    // Nadie se quita su propio «Gestionar perfiles» ni se desactiva a sí mismo.
    const me = (await admin<{ id: number }>("GET", "/v1/auth/me")).body.id;
    expect((await admin("PATCH", `/v1/profiles/${me}?scope=internal`, { name: "Admin Demo", role: "admin", permissions: [] })).status).toBe(403);
    expect((await admin("POST", `/v1/profiles/${me}/disable?scope=internal`, { reason: "Me voy de vacaciones" })).status).toBe(403);

    const log = (await (await ordersApi("admin@nexus.local"))<Activity>("GET", `/v1/activity?module=users&q=${encodeURIComponent(`E2E Interno ${stamp}`)}&limit=50`)).body.items
      .filter((i) => i.entity.type === "user" && i.entity.id === String(id));
    expect(log.map((i) => i.action).sort()).toEqual(["created", "password_reset", "updated"]);
    expect(log.find((i) => i.action === "updated")!.detailLines.join(" ")).toContain("Perfil");
  });
});

test.describe("Usuarios WeStorage · Fase 1: pantalla", () => {
  test("Administrador: menú, tabla del mockup y nuevo perfil sin correo con contraseña temporal", async ({ page }) => {
    await login(page, "admin@nexus.local");
    await sidebar(page).getByRole("button", { name: "Configuración" }).click();
    await expect(sidebar(page).getByRole("link", { name: "Ajustes generales" })).toHaveAttribute("aria-disabled", "true");
    await sidebar(page).getByRole("link", { name: /Usuarios WeStorage/ }).click();
    await expect(page.getByRole("heading", { name: "Usuarios WeStorage" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Perfiles WeStorage" })).toBeVisible();
    const table = page.locator(".clients-list");
    await expect(table.locator("th")).toHaveText(["", "Nombre completo", "Correo o usuario", "Rol", "Permisos", "Estado", "Fecha de creación"]);
    await expect(table.getByText("Usuario: seed.pedro.bodega")).toBeVisible();
    await expect(table.getByRole("row", { name: /Benjamín Cueto/ }).getByText("Inactivo")).toBeVisible();

    const stamp = Date.now();
    await page.getByRole("button", { name: "Nuevo perfil" }).click();
    const modal = page.locator(".form-modal");
    await modal.getByLabel("Nombre completo *").fill(`E2E Pantalla ${stamp}`);
    await modal.getByLabel("Rol *").selectOption("empaque");
    await modal.getByLabel("Despacho").check();
    await modal.getByLabel("Usuario sin correo").check();
    await modal.getByLabel("Nombre de usuario *").fill(`e2e.pant.${stamp}`);
    await modal.getByRole("button", { name: "Crear perfil" }).click();
    const temp = page.locator(".form-modal").filter({ hasText: "Contraseña temporal" });
    await expect(temp.locator("code")).not.toBeEmpty();
    await temp.getByRole("button", { name: "Listo" }).click();
    const panel = page.locator(".clients-panel");
    await expect(panel.getByRole("heading", { name: `Detalle de perfil E2E Pantalla ${stamp}` })).toBeVisible();
    await expect(panel.getByText(`e2e.pant.${stamp}`)).toBeVisible();
    await expect(panel.getByRole("button", { name: "Restablecer contraseña" })).toBeEnabled();
  });

  test("Administrador sin el permiso: solo lectura · Jefe: sin Configuración", async ({ browser }) => {
    const reader = await browser.newPage();
    await login(reader, "admin.lectura@nexus.local");
    await reader.goto("/configuracion/usuarios");
    await expect(reader.getByRole("heading", { name: "Perfiles WeStorage" })).toBeVisible();
    await expect(reader.getByRole("button", { name: "Nuevo perfil", exact: true })).toHaveCount(0);
    await reader.locator(".clients-list").getByRole("button", { name: /Ver detalle del perfil Esperanza Gómez/ }).click();
    await expect(reader.locator(".clients-panel").getByRole("button", { name: "Editar" })).toHaveCount(0);
    await reader.close();

    const jefe = await browser.newPage();
    await login(jefe, "jefe@nexus.local");
    await expect(sidebar(jefe).getByRole("button", { name: "Configuración" })).toHaveCount(0);
    await jefe.goto("/configuracion/usuarios");
    await jefe.waitForURL((url) => url.pathname === "/");
    await jefe.close();
  });
});

test.describe("Usuarios WeStorage · Fase 2: solicitudes", () => {
  test("API: el Administrador sin permiso solicita; con el permiso se aprueba; campana, correo y registro", async ({ browser }) => {
    const admin = await coreApi("admin@nexus.local");
    const reader = await coreApi("admin.lectura@nexus.local");
    const cliente = await coreApi("cliente1@nexus.local");
    const stamp = Date.now();

    // Campana en vivo del gestor.
    const page = await browser.newPage();
    await login(page, "admin@nexus.local");
    await page.goto("/ordenes/empaque");
    await expect(page.getByText("En vivo")).toBeVisible();

    const body = { kind: "new_profile", name: `E2E Solicitud ${stamp}`, username: `e2e.sol.${stamp}`, role: "empaque", permissions: ["despacho"], reason: "Nuevo operario de bodega" };
    expect((await cliente("POST", "/v1/profile-requests?scope=internal", body)).status).toBe(403);
    const res = await reader<Req>("POST", "/v1/profile-requests?scope=internal", body);
    expect(res.status).toBe(201);
    expect(res.body).toMatchObject({ scope: "internal", companyId: 0, profile: { username: `e2e.sol.${stamp}`, role: "empaque" } });
    // Una segunda pendiente igual se rechaza (índice con NULLS NOT DISTINCT).
    expect((await reader("POST", "/v1/profile-requests?scope=internal", body)).status).toBe(409);
    // El Cliente no la ve; por el alcance de clientes no existe.
    expect((await cliente("GET", "/v1/profile-requests?scope=internal")).status).toBe(403);
    expect((await admin("GET", `/v1/profile-requests/${res.body.id}`)).status).toBe(404);

    const bell = page.locator(".notification-bell > button");
    await expect(bell).toHaveAttribute("aria-label", /sin leer/);
    await bell.click();
    await expect(page.getByRole("link", { name: /Nueva solicitud de perfil WeStorage/ })).toHaveAttribute("href", `/configuracion/usuarios?tab=solicitudes&solicitud=${res.body.id}`);
    await page.close();
    await expect.poll(() => mailSubjects("admin@nexus.local")).toContain("[NexusWST] Nueva solicitud de perfil WeStorage");

    // Sin el permiso no se aprueba; con él, se crea el perfil y la contraseña temporal viene una vez.
    expect((await reader("POST", `/v1/profile-requests/${res.body.id}/approve?scope=internal`)).status).toBe(403);
    const ok = await admin<Req>("POST", `/v1/profile-requests/${res.body.id}/approve?scope=internal`);
    expect(ok.status).toBe(200);
    expect(ok.body.status).toBe("approved");
    expect(ok.body.temporaryPassword).toBeTruthy();
    const list = await admin<{ data: { username?: string; role: string; permissions: string[] }[] }>("GET", `/v1/profiles?scope=internal&q=e2e.sol.${stamp}`);
    expect(list.body.data).toMatchObject([{ username: `e2e.sol.${stamp}`, role: "empaque", permissions: ["despacho"] }]);
    await expect.poll(() => mailSubjects("admin.lectura@nexus.local")).toContain("[NexusWST] Tu solicitud de perfil fue aprobada");

    const log = (await (await ordersApi("admin@nexus.local"))<Activity>("GET", `/v1/activity?module=users&q=${encodeURIComponent(`E2E Solicitud ${stamp}`)}&limit=50`)).body.items;
    expect(log.map((i) => i.action)).toEqual(expect.arrayContaining(["request_created", "request_approved", "created"]));
  });

  test("UI: solicitar desactivación sin el permiso y rechazarla con el permiso", async ({ browser }) => {
    const reader = await browser.newPage();
    await login(reader, "admin.lectura@nexus.local");
    await reader.goto("/configuracion/usuarios");
    await expect(reader.getByRole("button", { name: "Solicitar nuevo perfil" })).toBeVisible();
    await reader.locator(".clients-list").getByRole("button", { name: /Ver detalle del perfil Esperanza Gómez/ }).click();
    await reader.locator(".clients-panel").getByRole("button", { name: "Solicitar desactivación" }).click();
    const dialog = reader.getByRole("dialog");
    await dialog.getByLabel(/Motivo/).fill("Termina su contrato");
    await dialog.getByRole("button", { name: "Enviar solicitud" }).click();
    await expect(reader.getByText(/Solicitud enviada/)).toBeVisible();
    await reader.close();

    const admin = await browser.newPage();
    await login(admin, "admin@nexus.local");
    await admin.goto("/configuracion/usuarios?tab=solicitudes");
    await expect(admin.locator(".clients-list th")).toHaveText(["Tipo", "Nombre perfil", "Rol", "Solicitado por", "Estado", "Fecha de solicitud"]);
    await expect(admin.locator(".clients-list").getByText("Rocío Fuentes")).toBeVisible(); // la del seed
    const row = admin.locator(".clients-list tr").filter({ hasText: "Esperanza Gómez" }).filter({ hasText: "Pendiente" });
    await row.getByRole("button").first().click();
    const panel = admin.locator(".clients-panel");
    await expect(panel.getByText("Desactivación")).toBeVisible();
    await panel.getByRole("button", { name: "Rechazar" }).click();
    const reject = admin.getByRole("dialog");
    await reject.getByLabel(/Motivo del rechazo/).fill("Sigue hasta fin de mes");
    await reject.getByRole("button", { name: "Rechazar" }).click();
    await expect(panel.getByText("Rechazada")).toBeVisible();
    await expect(panel.getByText("Sigue hasta fin de mes")).toBeVisible();
    await admin.close();
  });
});

import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist/**", "dist_keycloak/**", "node_modules/**", "public/keycloakify-dev-resources/**", "src/kc.gen.tsx"] },
  ...tseslint.configs.recommended,
);

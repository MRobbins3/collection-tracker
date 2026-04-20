/**
 * Ambient declaration so vue-tsc can resolve `.vue` SFC imports with the
 * `~/` alias in test files. Nuxt's baseline types cover component
 * auto-imports from templates but don't describe arbitrary `~/components/…`
 * imports used by the test suite.
 */
declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}

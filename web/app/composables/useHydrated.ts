import { onMounted, ref } from "vue";

/**
 * False during SSR and the hydrating render, true once this component has
 * mounted in the browser.
 *
 * Use it to disable an input that is server-rendered but only works after
 * hydration, e.g. a search box bound with v-model. Text typed into the SSR
 * markup before hydration is lost: the input event fires before Vue attaches
 * its listener, then hydration resets the DOM value to the model's (empty)
 * value. Rendering `:disabled="!hydrated"` makes the SSR markup and the first
 * client render agree (no hydration mismatch) and enables the box the moment
 * typing would work. Playwright waits for an input to be enabled before
 * filling it, so e2e specs need no hydration sleeps.
 */
export function useHydrated() {
  const hydrated = ref(false);
  onMounted(() => {
    hydrated.value = true;
  });
  return hydrated;
}

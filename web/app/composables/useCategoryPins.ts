/**
 * Add-to-collection state for the public category pages: which categories
 * are in your overview, and pin/unpin actions that refresh it. Signed-out
 * visitors get "none" for everything and no requests (client-only fetch).
 */
export function useCategoryPins() {
  const { isSignedIn } = useAuth();
  const { data, refresh } = useMyOverviewCategories();
  const { pin, unpin } = useOverviewActions();
  const busySlug = ref<string | null>(null);
  const error = ref<string | null>(null);

  const bySlug = computed(() => new Map(data.value.categories.map((c) => [c.slug, c])));

  function stateOf(slug: string): "none" | "pinned" | "collecting" {
    const c = bySlug.value.get(slug);
    if (!c) return "none";
    return c.collection_count > 0 ? "collecting" : "pinned";
  }

  async function run(slug: string, action: (s: string) => Promise<void>, failure: string) {
    busySlug.value = slug;
    error.value = null;
    try {
      await action(slug);
      await refresh();
    } catch (e) {
      console.error(e);
      error.value = failure;
    } finally {
      busySlug.value = null;
    }
  }

  return {
    isSignedIn,
    busySlug,
    error,
    stateOf,
    add: (slug: string) => run(slug, pin, "Couldn’t add that category. Please try again."),
    remove: (slug: string) => run(slug, unpin, "Couldn’t remove that category. Please try again."),
  };
}

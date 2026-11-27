import type { CategoriesResponse, Category, CategoryDraft, LibraryField, Submission } from "~/types/api";

/**
 * The server render has no session cookie, so it only sees approved
 * categories. Your own categories (ADR 0013) arrive when the signed-in
 * state resolves after hydration: these fetches watch isSignedIn and load
 * again then, which keeps SSR and the first client render identical.
 */

/**
 * `query` should already be normalized and debounced. The static key is
 * fine: `watch` refetches under the same key whenever the query changes.
 */
export function useCategoriesList(query: Readonly<Ref<string>>) {
  const api = useApi();
  const { isSignedIn } = useAuth();
  return useAsyncData<CategoriesResponse>(
    "categories",
    () => api.get<CategoriesResponse>("/categories", query.value ? { q: query.value } : undefined),
    { watch: [query, isSignedIn] },
  );
}

/** One category, or null when it doesn't exist or isn't visible to you. */
export function useCategory(slug: string) {
  const api = useApi();
  const { isSignedIn } = useAuth();
  return useAsyncData<Category | null>(
    `category:${slug}`,
    async () => {
      try {
        return await api.get<Category>(`/categories/${slug}`);
      } catch (e) {
        if (statusOf(e) === 404) return null;
        throw e;
      }
    },
    { watch: [isSignedIn] },
  );
}

/** The field library the category builder picks from (public, client only). */
export function useCategoryFields() {
  const api = useApi();
  return useAsyncData<LibraryField[]>(
    "category-fields",
    async () => (await api.get<{ fields: LibraryField[] }>("/category-fields")).fields,
    { server: false, default: () => [] },
  );
}

/** Create, edit, and submit your own categories. */
export function useMyCategoryActions() {
  const { publicBaseURL } = useApi();

  async function create(draft: CategoryDraft): Promise<Category> {
    return await $fetch<Category>(`${publicBaseURL}/me/categories`, {
      method: "POST",
      credentials: "include",
      body: draft,
    });
  }

  async function update(slug: string, draft: CategoryDraft): Promise<Category> {
    return await $fetch<Category>(`${publicBaseURL}/me/categories/${slug}`, {
      method: "PATCH",
      credentials: "include",
      body: draft,
    });
  }

  async function submit(slug: string, note?: string): Promise<Submission> {
    return await $fetch<Submission>(`${publicBaseURL}/me/categories/${slug}/submit`, {
      method: "POST",
      credentials: "include",
      body: note ? { note } : {},
    });
  }

  return { create, update, submit };
}

function statusOf(e: unknown): number | undefined {
  const err = e as { statusCode?: number; status?: number } | null;
  return err?.statusCode ?? err?.status;
}

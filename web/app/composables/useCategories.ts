import type { CategoriesResponse, Category } from "~/types/api";

/**
 * `query` should already be normalized and debounced. The static key is
 * fine: `watch` refetches under the same key whenever the query changes.
 */
export function useCategoriesList(query: Readonly<Ref<string>>) {
  const api = useApi();
  return useAsyncData<CategoriesResponse>(
    "categories",
    () => api.get<CategoriesResponse>("/categories", query.value ? { q: query.value } : undefined),
    { watch: [query] },
  );
}

export function useCategory(slug: string) {
  const api = useApi();
  return useAsyncData<Category>(`category:${slug}`, () => api.get<Category>(`/categories/${slug}`));
}

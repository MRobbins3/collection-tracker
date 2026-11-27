import type { CategoriesResponse, Category } from "~/types/api";

export function useCategoriesList(query: Ref<string>) {
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

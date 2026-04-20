export interface Category {
  id: string;
  slug: string;
  name: string;
  description?: string;
  attribute_schema: Record<string, unknown>;
}

export interface CategoriesResponse {
  categories: Category[];
}

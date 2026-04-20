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

export interface User {
  id: string;
  email: string;
  display_name: string;
  created_at: string;
}

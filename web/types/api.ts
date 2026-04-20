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

export interface Collection {
  id: string;
  category_id: string;
  category_slug: string;
  category_name: string;
  name: string;
  item_count: number;
  created_at: string;
  updated_at: string;
}

export interface CollectionsResponse {
  collections: Collection[];
}

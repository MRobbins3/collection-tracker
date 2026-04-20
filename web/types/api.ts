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

export interface Item {
  id: string;
  collection_id: string;
  catalog_entry_id?: string;
  name: string;
  quantity: number;
  condition?: string;
  attributes: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ItemsResponse {
  items: Item[];
}

export interface CatalogEntry {
  id: string;
  category_id: string;
  name: string;
  description?: string;
  attributes: Record<string, unknown>;
  source: "seed" | "user_submitted" | "import";
  status: "pending" | "approved" | "rejected";
}

export interface CatalogSearchResponse {
  entries: CatalogEntry[];
}

// Shape of a category's JSON schema property after we extend it with
// human labels. Used by the dynamic form renderer.
export interface AttributeProperty {
  type: "string" | "integer" | "number" | "boolean";
  title?: string;
  description?: string;
  minimum?: number;
  maximum?: number;
}

export interface AttributeSchema {
  type: "object";
  properties: Record<string, AttributeProperty>;
  additionalProperties?: boolean;
}

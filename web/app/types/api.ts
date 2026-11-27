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
  /** Public, unique, lowercase username; the profile lives at /u/<handle>. */
  handle: string;
  /** Palette color name for the initials avatar (see utils/avatar.ts). */
  avatar_color: string;
  created_at: string;
}

/** Who can see a user's collections (ADR 0009). Shown via utils/privacy.ts labels, never raw. */
export type CollectionVisibility = "public" | "followers_and_friends" | "friends_only" | "private";

/** GET/PATCH /me/profile: your own profile plus privacy settings. */
export interface Profile {
  handle: string;
  display_name: string;
  bio: string;
  avatar_color: string;
  collection_visibility: CollectionVisibility;
  accept_friend_requests: boolean;
  accept_follow_requests: boolean;
}

/** The parts of a profile other people see. */
export interface PublicProfile {
  handle: string;
  display_name: string;
  bio: string;
  avatar_color: string;
}

/** GET /users/{handle}. Hidden or unknown profiles are a 404 instead. */
export interface PublicProfileResponse {
  profile: PublicProfile;
  is_self: boolean;
  collection_visibility: CollectionVisibility;
  collections_visible: boolean;
  /** Empty when collections_visible is false. */
  collections: Collection[];
}

/** GET /users/{handle}/collections/{id}: one collection, read-only. */
export interface PublicCollectionResponse {
  owner: PublicProfile;
  is_self: boolean;
  collection: Collection;
  items: Item[];
}

/** GET /me/handle-availability?handle= */
export interface HandleAvailability {
  /** The normalized handle that was checked. */
  handle: string;
  available: boolean;
  /** Why it isn't available; "" when it is. */
  message: string;
}

/** Validation error body: a summary plus a message per field. */
export interface FieldErrors {
  error: string;
  fields?: Record<string, string>;
}

/** Which sign-in providers the API offers (GET /auth/providers). */
export interface AuthProviders {
  google: boolean;
  dev: boolean;
}

/** A seeded test user offered by the local-only dev sign-in picker. */
export interface DevPersona {
  /** Sent to the API to sign in; never shown in the UI. */
  handle: string;
  display_name: string;
  email: string;
  collection_count: number;
  item_count: number;
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

/** One hit from GET /me/items?q= — search across your own items. */
export interface ItemSearchResult {
  id: string;
  name: string;
  quantity: number;
  condition?: string;
  collection_id: string;
  collection_name: string;
  category_slug: string;
  category_name: string;
  /** Set when the hit came from an attribute value, not the name. `label` is human-readable. */
  matched_field?: { label: string; value: string };
}

export interface ItemSearchResponse {
  items: ItemSearchResult[];
  /** True when more items matched than the API returns in one response. */
  has_more: boolean;
}

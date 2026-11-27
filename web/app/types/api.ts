export interface Category {
  id: string;
  slug: string;
  name: string;
  description?: string;
  attribute_schema: Record<string, unknown>;
  /** The attribute that, with the name, tells catalog entries apart (e.g. set number). */
  catalog_identifier?: string;
  /**
   * "approved" for every category on the public list. A category someone
   * built stays private, pending (waiting for review), or rejected until an
   * admin approves it, and only its creator sees it until then.
   */
  status?: CategoryStatus;
  /** You built this category. */
  mine?: boolean;
  /** For your own category: the admin's reason on its latest review. */
  review_reason?: string;
}

export type CategoryStatus = "private" | "pending" | "approved" | "rejected";

/** A reusable field from the library (GET /category-fields). */
export interface LibraryField {
  /** "<type>:<key>": what the builder sends back. */
  id: string;
  key: string;
  title: string;
  type: "string" | "integer" | "number";
  description?: string;
  minimum?: number;
  /** Known options (a pick list). Empty means free text. */
  options: string[];
  /** The categories that use this field, each with the options it offers. */
  sources: { slug: string; name: string; options?: string[] }[];
}

/** Body of POST /me/categories and PATCH /me/categories/{slug}. */
export interface CategoryDraft {
  name: string;
  description?: string;
  fields: { id: string; options: string[] }[];
  /** The id of the chosen field that tells items apart, if any. */
  catalog_identifier?: string;
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
  /** null for almost everyone; "admin" shows the moderation pages. The API enforces it. */
  role?: UserRole | null;
  created_at: string;
}

export type UserRole = "admin";

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
  /** Where you stand with this person. null when signed out or it's you. */
  relationship: ViewerRelationship | null;
}

/** A friendship from your side (ADR 0014). */
export type FriendStatus = "none" | "request_sent" | "request_received" | "friends";
/** One direction of following. Every follow needs approval. */
export type FollowStatus = "none" | "requested" | "following";

/** The relationship block of GET /users/{handle}. */
export interface ViewerRelationship {
  friend: FriendStatus;
  /** You → them. */
  following: FollowStatus;
  /** Them → you. */
  follows_you: FollowStatus;
  accepts_friend_requests: boolean;
  accepts_follow_requests: boolean;
}

/** The public card fields of a person (people search returns only these). */
export interface PersonCard {
  handle: string;
  display_name: string;
  avatar_color: string;
}

/** Someone in a friends, followers, or requests list. */
export interface Person extends PersonCard {
  /** When the friendship, follow, or request began. */
  since: string;
}

/** The tabs of the People page. */
export type PeopleTab = "friends" | "followers" | "following" | "requests";

/** GET /me/requests: everything pending to or from you. */
export interface RequestsResponse {
  friend_incoming: Person[];
  friend_outgoing: Person[];
  follow_incoming: Person[];
  follow_outgoing: Person[];
}

/** GET /users/{handle}/collections/{id}: one collection, read-only. */
export interface PublicCollectionResponse {
  owner: PublicProfile;
  is_self: boolean;
  collection: Collection;
  /** The collection's category (even one its owner built and hasn't published). */
  category: {
    slug: string;
    name: string;
    attribute_schema: Record<string, unknown>;
    /** On the public list. */
    public: boolean;
    /** Only when you're the owner. */
    status?: CategoryStatus;
  };
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

/** An approved entry in the global catalog (ADR 0004): a thing that exists in the world. */
export interface CatalogEntry {
  id: string;
  category_id: string;
  category_slug: string;
  category_name: string;
  name: string;
  /** Value of the category's identifier attribute (set number, artist...), when it has one. */
  identifier?: string;
  description?: string;
  attributes: Record<string, unknown>;
  source: "seed" | "user_submitted" | "import";
  status: "pending" | "approved" | "rejected";
}

/** GET /catalog/entries: one page of a category's approved entries. */
export interface CatalogPage {
  entries: CatalogEntry[];
  /** Entries matching the search across every page. */
  total: number;
  has_more: boolean;
}

/** Why an entry was suggested for a free-form item. Shown via utils/catalog.ts labels, never raw. */
export type CatalogMatch = "name_and_identifier" | "identifier" | "name" | "similar_name";

export interface CatalogSuggestion extends CatalogEntry {
  match: CatalogMatch;
}

/** GET /me/collections/{id}/items/{itemID}/catalog-suggestions */
export interface CatalogSuggestionsResponse {
  suggestions: CatalogSuggestion[];
}

/** One of your items linked to a catalog entry. */
export interface CatalogOwnership {
  catalog_entry_id: string;
  item_id: string;
  collection_id: string;
  collection_name: string;
}

/** GET /me/catalog-ownership?category_slug= */
export interface CatalogOwnershipResponse {
  owned: CatalogOwnership[];
}

// Shape of a category's JSON schema property after we extend it with
// human labels. Used by the dynamic form renderer.
export interface AttributeProperty {
  type: "string" | "integer" | "number" | "boolean";
  title?: string;
  description?: string;
  minimum?: number;
  maximum?: number;
  /** A pick list: the value must be one of these. */
  enum?: string[];
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

/** How the collection overview orders items. Shown via utils/overview.ts labels, never raw. */
export type OverviewSort = "newest" | "oldest" | "name_asc" | "name_desc" | "category";

/** A category in your overview: one you have collections in, or one you added (pinned). */
export interface OverviewCategory {
  slug: string;
  name: string;
  /** Item rows across your collections in this category (not the sum of quantities). */
  item_count: number;
  collection_count: number;
  /** You added it to your overview yourself. */
  pinned: boolean;
}

/** One of your items, with where it lives. */
export interface OverviewItem {
  id: string;
  name: string;
  quantity: number;
  condition?: string;
  collection_id: string;
  collection_name: string;
  category_slug: string;
  category_name: string;
  created_at: string;
}

/** GET /me/overview */
export interface OverviewResponse {
  categories: OverviewCategory[];
  show_empty_categories: boolean;
  items: OverviewItem[];
  /** Items matching the filter across every page. */
  total: number;
  has_more: boolean;
}

// ---- Catalog suggestions and moderation (ADR 0012) ----------------------

/** What a moderation request asks admins to review. Shown via utils/moderation.ts labels. */
export type ModerationKind = "catalog_entry" | "catalog_correction" | "category";
export type ModerationStatus = "pending" | "approved" | "rejected" | "merged";

/** Catalog entry fields in a correction, an admin's edits, or the audit record. */
export interface EntryFields {
  name?: string;
  /** "" clears the description. */
  description?: string;
  attributes?: Record<string, unknown>;
}

/** One of your own suggestions or corrections (GET /me/catalog-submissions). */
export interface Submission {
  id: string;
  kind: ModerationKind;
  status: ModerationStatus;
  category_slug: string;
  category_name: string;
  name: string;
  identifier?: string;
  attributes: Record<string, unknown>;
  proposed?: EntryFields;
  note?: string;
  /** The admin's explanation (rejections, merges). */
  reason?: string;
  decided_at?: string;
  created_at: string;
  /** The public catalog entry this led to, once there is one. */
  entry_id?: string;
  merged_into_name?: string;
}

export interface SubmissionsResponse {
  submissions: Submission[];
}

export type SubmitOutcome = "submitted" | "already_in_catalog" | "already_submitted" | "pending_review";

/** POST /me/catalog-submissions */
export interface SubmitResult {
  outcome: SubmitOutcome;
  entry?: CatalogEntry;
  submission?: Submission;
  item_linked: boolean;
}

export interface UserRef {
  display_name: string;
  handle: string;
}

export interface QueueItem {
  id: string;
  kind: ModerationKind;
  category_slug: string;
  category_name: string;
  name: string;
  identifier?: string;
  note?: string;
  submitter: UserRef;
  created_at: string;
}

/** GET /admin/moderation */
export interface QueuePage {
  requests: QueueItem[];
  total: number;
  has_more: boolean;
}

/** GET /admin/moderation/{id} */
export interface ModerationRequest {
  id: string;
  kind: ModerationKind;
  status: ModerationStatus;
  category_slug: string;
  category_name: string;
  /** The submitted entry, or for a correction the entry as it is now. Absent for a category. */
  entry?: CatalogEntry;
  /** A category asking to join the public list, as it is now (kind "category"). */
  category?: Category;
  proposed?: EntryFields;
  previous?: EntryFields;
  applied?: EntryFields;
  note?: string;
  submitter: UserRef;
  decided_by?: UserRef;
  decided_at?: string;
  reason?: string;
  merged_into?: CatalogEntry;
  created_at: string;
}

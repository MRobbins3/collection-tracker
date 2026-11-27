import type { AttributeSchema, CatalogMatch, Category } from "~/types/api";

/**
 * The human label for a category's catalog identifier attribute ("Set
 * number", "ISBN"), from its attribute schema; null when the category has
 * none. Never the raw key (CLAUDE.md, "No raw schema keys in UI").
 */
export function identifierTitle(category: Pick<Category, "catalog_identifier" | "attribute_schema"> | null | undefined): string | null {
  const key = category?.catalog_identifier;
  if (!key) return null;
  const schema = category.attribute_schema as Partial<AttributeSchema> | undefined;
  return schema?.properties?.[key]?.title ?? humanizeKey(key);
}

/** "set_number" → "Set number". Last resort when a schema property has no title. */
export function humanizeKey(key: string): string {
  const spaced = key.replace(/_/g, " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

/**
 * Lowercase a label's first letter for use mid-sentence ("Set number" →
 * "set number") but keep acronyms ("ISBN", "Catalog ID" → "catalog ID").
 */
export function midSentence(label: string): string {
  if (label.length > 1 && label.charAt(1) === label.charAt(1).toUpperCase() && /[A-Z]/.test(label.charAt(1))) {
    return label;
  }
  return label.charAt(0).toLowerCase() + label.slice(1);
}

/** Why a catalog entry was suggested, in words: "Same name and set number". */
export function matchLabel(match: CatalogMatch, idTitle: string | null): string {
  const id = midSentence(idTitle ?? "identifier");
  switch (match) {
    case "name_and_identifier":
      return `Same name and ${id}`;
    case "identifier":
      return `Same ${id}`;
    case "name":
      return "Same name";
    default:
      return "Similar name";
  }
}

/** Where a catalog entry's detail page lives. */
export function catalogEntryPath(categorySlug: string, entryId: string): string {
  return `/categories/${categorySlug}/catalog/${entryId}`;
}

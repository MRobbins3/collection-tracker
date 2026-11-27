import { collapseWhitespace } from "~/utils/text";
import type { AttributeSchema, Category, CategoryDraft, CategoryStatus, LibraryField } from "~/types/api";

/**
 * Pure helpers for the "Create a category" builder (ADR 0013). The API is
 * the authority: it composes the real schema from the same choices
 * (category.Compose) and normalizes options again. These keep the builder's
 * preview and instant feedback in step with it.
 */

export const CATEGORY_NAME_MAX = 60;
export const CATEGORY_DESCRIPTION_MAX = 300;
export const MAX_FIELDS = 20;
export const MAX_OPTIONS = 50;
export const OPTION_MAX = 60;

/** A field the user picked, with the option list they want (text fields only). */
export interface FieldSelection {
  field: LibraryField;
  options: string[];
}

export interface FieldGroup {
  source: { slug: string; name: string };
  fields: LibraryField[];
}

/**
 * The library grouped by the category each field comes from, filtered by a
 * search over field titles, descriptions, and category names. A field used
 * by several categories appears under each of them (it's the same field).
 */
export function groupFieldsBySource(fields: LibraryField[], query: string): FieldGroup[] {
  const q = collapseWhitespace(query).toLowerCase();
  const groups = new Map<string, FieldGroup>();
  for (const f of fields) {
    for (const src of f.sources) {
      const matches =
        !q ||
        f.title.toLowerCase().includes(q) ||
        (f.description ?? "").toLowerCase().includes(q) ||
        src.name.toLowerCase().includes(q);
      if (!matches) continue;
      let g = groups.get(src.slug);
      if (!g) {
        g = { source: src, fields: [] };
        groups.set(src.slug, g);
      }
      g.fields.push(f);
    }
  }
  return [...groups.values()].sort((a, b) => a.source.name.localeCompare(b.source.name));
}

export type AddOptionResult =
  | { kind: "added"; options: string[]; value: string }
  | { kind: "exists"; options: string[]; value: string }
  | { kind: "invalid"; options: string[]; message: string };

/**
 * Adds a typed option: trimmed and collapsed; a value matching an existing
 * option case-insensitively isn't added again (the existing spelling is
 * kept, and reported so the UI can say so). Casing is otherwise kept as
 * typed; the UI offers Title Case separately (suggestTitleCase).
 */
export function addOption(options: string[], typed: string): AddOptionResult {
  const value = collapseWhitespace(typed);
  if (!value) return { kind: "invalid", options, message: "Type an option first." };
  if ([...value].length > OPTION_MAX) {
    return { kind: "invalid", options, message: `Options are at most ${OPTION_MAX} characters.` };
  }
  const existing = options.find((o) => o.toLowerCase() === value.toLowerCase());
  if (existing !== undefined) return { kind: "exists", options, value: existing };
  if (options.length >= MAX_OPTIONS) {
    return { kind: "invalid", options, message: `A field can have at most ${MAX_OPTIONS} options.` };
  }
  return { kind: "added", options: [...options, value], value };
}

/** The item form schema the selections would produce: same shape the API stores. */
export function previewSchema(selections: FieldSelection[]): AttributeSchema {
  const properties: AttributeSchema["properties"] = {};
  for (const { field, options } of selections) {
    properties[field.key] = {
      type: field.type,
      title: field.title,
      ...(field.description ? { description: field.description } : {}),
      ...(field.minimum !== undefined ? { minimum: field.minimum } : {}),
      ...(field.type === "string" && options.length > 0 ? { enum: options } : {}),
    };
  }
  return { type: "object", properties, additionalProperties: false };
}

/** The request body for POST /me/categories (or PATCH). */
export function toDraft(input: {
  name: string;
  description: string;
  selections: FieldSelection[];
  identifierId: string;
}): CategoryDraft {
  const description = collapseWhitespace(input.description);
  return {
    name: collapseWhitespace(input.name),
    ...(description ? { description } : {}),
    fields: input.selections.map((s) => ({ id: s.field.id, options: s.field.type === "string" ? s.options : [] })),
    ...(input.identifierId ? { catalog_identifier: input.identifierId } : {}),
  };
}

/**
 * The selections an existing category was built from, for editing: each
 * schema property becomes a field (from the library when it's there, or
 * rebuilt from the schema so fields only this category has stay editable).
 */
export function selectionsFromCategory(category: Category, library: LibraryField[]): FieldSelection[] {
  const props = (category.attribute_schema as Partial<AttributeSchema>).properties ?? {};
  const byId = new Map(library.map((f) => [f.id, f]));
  return Object.entries(props)
    .filter(([, p]) => p.type === "string" || p.type === "integer" || p.type === "number")
    .map(([key, p]) => {
      const id = `${p.type}:${key}`;
      const field: LibraryField = byId.get(id) ?? {
        id,
        key,
        title: p.title ?? key,
        type: p.type as LibraryField["type"],
        description: p.description,
        minimum: p.minimum,
        options: [],
        sources: [],
      };
      return { field, options: [...(p.enum ?? [])] };
    });
}

/** Problems the API would refuse, worded like it, or null. */
export function draftProblem(name: string, description: string, selections: FieldSelection[]): string | null {
  const n = collapseWhitespace(name);
  if (!n || [...n].length > CATEGORY_NAME_MAX) return `Give the category a name of 1–${CATEGORY_NAME_MAX} characters.`;
  if ([...collapseWhitespace(description)].length > CATEGORY_DESCRIPTION_MAX) {
    return `Keep the description to ${CATEGORY_DESCRIPTION_MAX} characters.`;
  }
  if (selections.length === 0) return "Pick at least one field.";
  if (selections.length > MAX_FIELDS) return `A category can have at most ${MAX_FIELDS} fields.`;
  const keys = new Set<string>();
  for (const s of selections) {
    if (keys.has(s.field.key)) return `You picked two fields called “${s.field.title}”. Keep one of them.`;
    keys.add(s.field.key);
  }
  return null;
}

/** A category's status in words; null for an ordinary public category. */
export function categoryStatusLabel(status: CategoryStatus | undefined): string | null {
  switch (status) {
    case "private":
      return "Private";
    case "pending":
      return "Waiting for review";
    case "rejected":
      return "Not approved";
    default:
      return null;
  }
}

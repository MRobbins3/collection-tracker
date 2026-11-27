import { humanizeKey } from "~/utils/catalog";
import type {
  AttributeSchema,
  CatalogEntry,
  EntryFields,
  ModerationKind,
  ModerationStatus,
  SubmitResult,
} from "~/types/api";

/** "New entry" / "Correction" / "New category": never the raw kind. */
export function kindLabel(kind: ModerationKind): string {
  if (kind === "category") return "New category";
  return kind === "catalog_correction" ? "Correction" : "New entry";
}

/** A request's status in words, from the submitter's point of view. */
export function statusLabel(status: ModerationStatus, kind: ModerationKind): string {
  switch (status) {
    case "pending":
      return "Waiting for review";
    case "approved":
      if (kind === "category") return "On the public list";
      return kind === "catalog_correction" ? "Correction applied" : "Added to the catalog";
    case "merged":
      return "Already in the catalog";
    default:
      return "Not accepted";
  }
}

/** What a suggestion did, as a sentence for the person who sent it. */
export function outcomeMessage(result: SubmitResult): string {
  switch (result.outcome) {
    case "already_in_catalog":
      return `“${result.entry?.name ?? "That"}” is already in the catalog.${
        result.item_linked ? " Your item is now linked to it." : ""
      }`;
    case "already_submitted":
      return "You already suggested this. It’s waiting for review in My submissions.";
    case "pending_review":
      return "Someone already suggested this. It’s waiting for review.";
    default:
      return "Thanks! An admin will review your suggestion. You can follow it in My submissions.";
  }
}

/** A displayable value: blank becomes an em dash. */
export function formatValue(v: unknown): string {
  if (v === undefined || v === null || v === "") return "—";
  return String(v);
}

export interface DiffRow {
  key: string;
  label: string;
  before: string;
  after: string;
}

/**
 * The fields `changes` would change on `entry`, labelled for people (schema
 * titles, never raw keys). Attributes are compared key by key; a key the
 * change leaves out counts as removed, since corrections send the whole
 * attribute set.
 */
export function diffRows(
  schema: Partial<AttributeSchema> | undefined,
  entry: Pick<CatalogEntry, "name" | "description" | "attributes">,
  changes: EntryFields,
): DiffRow[] {
  const rows: DiffRow[] = [];
  const push = (key: string, label: string, before: unknown, after: unknown) => {
    const b = formatValue(before);
    const a = formatValue(after);
    if (b !== a) rows.push({ key, label, before: b, after: a });
  };
  if (changes.name !== undefined) push("name", "Name", entry.name, changes.name);
  if (changes.description !== undefined) push("description", "Description", entry.description, changes.description);
  if (changes.attributes !== undefined) {
    const props = schema?.properties ?? {};
    const keys = [
      ...Object.keys(props),
      ...new Set([...Object.keys(entry.attributes ?? {}), ...Object.keys(changes.attributes)].filter((k) => !(k in props))),
    ];
    for (const key of keys) {
      push(
        `attributes.${key}`,
        props[key]?.title ?? humanizeKey(key),
        entry.attributes?.[key],
        changes.attributes[key],
      );
    }
  }
  return rows;
}

/** The API's error message from a failed $fetch, or a fallback. */
export function apiErrorMessage(e: unknown, fallback: string): string {
  const data = (e as { data?: { error?: unknown } } | null)?.data;
  return typeof data?.error === "string" && data.error ? data.error : fallback;
}

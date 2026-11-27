import type {
  EntryFields,
  ModerationKind,
  ModerationRequest,
  QueuePage,
  Submission,
  SubmissionsResponse,
  SubmitResult,
} from "~/types/api";

/** Requests per page in the admin queue. */
export const MODERATION_PAGE_SIZE = 25;

export interface SubmitEntryInput {
  category_slug: string;
  name: string;
  description?: string;
  attributes?: Record<string, unknown>;
  /** Your free-form item this came from: it's linked once the entry is in the catalog. */
  item_id?: string;
}

export interface CorrectionInput extends EntryFields {
  catalog_entry_id: string;
  note?: string;
}

/** Your own catalog suggestions and corrections, with outcomes (signed in, client only). */
export function useMySubmissions() {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions();
  return useAsyncData<SubmissionsResponse>(
    "me:catalog-submissions",
    async () =>
      isSignedIn.value ? api.get<SubmissionsResponse>("/me/catalog-submissions") : { submissions: [] },
    { server, watch, default: () => ({ submissions: [] }) },
  );
}

/** Suggest a new catalog entry, or a correction to an existing one. */
export function useSubmissionActions() {
  const { publicBaseURL } = useApi();

  async function submitEntry(input: SubmitEntryInput): Promise<SubmitResult> {
    return await $fetch<SubmitResult>(`${publicBaseURL}/me/catalog-submissions`, {
      method: "POST",
      credentials: "include",
      body: input,
    });
  }

  async function proposeCorrection(input: CorrectionInput): Promise<Submission> {
    return await $fetch<Submission>(`${publicBaseURL}/me/catalog-corrections`, {
      method: "POST",
      credentials: "include",
      body: input,
    });
  }

  return { submitEntry, proposeCorrection };
}

/**
 * Admin only: the moderation queue and decisions. The API refuses
 * non-admins (403), so the pages only render these for admins.
 */
export function useModerationAdmin() {
  const api = useApi();
  const { publicBaseURL } = api;
  const base = `${publicBaseURL}/admin/moderation`;

  async function queue(filters: { kind?: ModerationKind | ""; categorySlug?: string; offset?: number }): Promise<QueuePage> {
    return await api.get<QueuePage>("/admin/moderation", {
      kind: filters.kind || undefined,
      category_slug: filters.categorySlug || undefined,
      limit: MODERATION_PAGE_SIZE,
      offset: filters.offset || undefined,
    });
  }

  async function get(id: string): Promise<ModerationRequest> {
    return await api.get<ModerationRequest>(`/admin/moderation/${id}`);
  }

  async function approve(id: string, edits?: EntryFields): Promise<ModerationRequest> {
    return await $fetch<ModerationRequest>(`${base}/${id}/approve`, {
      method: "POST",
      credentials: "include",
      body: edits ?? {},
    });
  }

  async function reject(id: string, reason: string): Promise<ModerationRequest> {
    return await $fetch<ModerationRequest>(`${base}/${id}/reject`, {
      method: "POST",
      credentials: "include",
      body: { reason },
    });
  }

  async function merge(id: string, catalogEntryId: string, reason?: string): Promise<ModerationRequest> {
    return await $fetch<ModerationRequest>(`${base}/${id}/merge`, {
      method: "POST",
      credentials: "include",
      body: { catalog_entry_id: catalogEntryId, reason: reason || undefined },
    });
  }

  return { queue, get, approve, reject, merge };
}

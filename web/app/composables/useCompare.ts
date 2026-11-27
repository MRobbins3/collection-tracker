import type { CompareBucket, CompareResponse } from "~/types/api";

export interface CompareParams {
  /** Category slug; omitted = the category picker only. */
  category?: string;
  bucket?: CompareBucket;
  offset?: number;
  limit?: number;
}

/**
 * Compare your collection with someone else's (GET /users/{handle}/compare).
 * Signed-in only. The API applies the privacy rule: a person whose
 * collections you can't see is a 404, which resolves to "hidden" here; a
 * category that doesn't exist (or that you can't see) resolves to
 * "unknown_category".
 */
export function useCompareActions() {
  const api = useApi();

  async function fetchCompare(
    handle: string,
    params: CompareParams,
  ): Promise<CompareResponse | "hidden" | "unknown_category"> {
    try {
      return await api.get<CompareResponse>(`/users/${encodeURIComponent(handle)}/compare`, {
        category: params.category || undefined,
        bucket: params.bucket,
        offset: params.offset,
        limit: params.limit,
      });
    } catch (e) {
      const status = (e as { statusCode?: number; status?: number })?.statusCode ?? (e as { status?: number })?.status;
      if (status === 404) {
        const body = (e as { data?: { error?: string } })?.data;
        return body?.error === "category not found" ? "unknown_category" : "hidden";
      }
      throw e;
    }
  }

  return { fetchCompare };
}

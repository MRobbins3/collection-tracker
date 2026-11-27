import { describe, it, expect, vi, beforeEach } from "vitest";
import { useCompareActions } from "~/composables/useCompare";

let get: ReturnType<typeof vi.fn>;

beforeEach(() => {
  get = vi.fn();
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useApi = () => ({ get, publicBaseURL: "http://api" });
});

const notFound = (data: Record<string, string>) => Object.assign(new Error("404"), { statusCode: 404, data });

describe("useCompareActions", () => {
  it("passes the query through, leaving out an empty category", async () => {
    get.mockResolvedValue({ items: [] });
    await useCompareActions().fetchCompare("No ah", { category: "", bucket: "neither", offset: 50, limit: 50 });
    expect(get).toHaveBeenCalledWith("/users/No%20ah/compare", {
      category: undefined,
      bucket: "neither",
      offset: 50,
      limit: 50,
    });
  });

  it("tells an unknown category apart from a hidden profile by its code", async () => {
    get.mockRejectedValueOnce(notFound({ error: "category not found", code: "category_not_found" }));
    expect(await useCompareActions().fetchCompare("noah", { category: "x" })).toBe("unknown_category");

    // The message alone doesn't count; only the code does.
    get.mockRejectedValueOnce(notFound({ error: "category not found" }));
    expect(await useCompareActions().fetchCompare("noah", { category: "x" })).toBe("hidden");

    get.mockRejectedValueOnce(notFound({ error: "profile not found" }));
    expect(await useCompareActions().fetchCompare("noah", {})).toBe("hidden");
  });

  it("rethrows other errors", async () => {
    get.mockRejectedValueOnce(Object.assign(new Error("500"), { statusCode: 500 }));
    await expect(useCompareActions().fetchCompare("noah", {})).rejects.toThrow("500");
  });
});

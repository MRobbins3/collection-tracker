import { describe, it, expect, vi } from "vitest";
import { nextTick, ref } from "vue";
import { useShowMore, type ItemsPage } from "~/composables/useShowMore";

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

describe("useShowMore", () => {
  it("appends the next page at the right offset", async () => {
    const first = ref<ItemsPage<string> & { extra?: boolean }>({ items: ["a", "b"], has_more: true });
    const fetchPage = vi.fn(async () => ({ items: ["c"], has_more: false }));
    const p = useShowMore(first, fetchPage);
    await p.loadMore();
    expect(fetchPage).toHaveBeenCalledWith(2);
    expect(p.items.value).toEqual(["a", "b", "c"]);
    expect(p.hasMore.value).toBe(false);
  });

  it("drops a page that arrives after the first page was replaced", async () => {
    const first = ref<ItemsPage<string>>({ items: ["a", "b"], has_more: true });
    const d = deferred<ItemsPage<string>>();
    const p = useShowMore(first, () => d.promise);
    const pending = p.loadMore();
    expect(p.loadingMore.value).toBe(true);

    first.value = { items: ["z"], has_more: false }; // sort/filter changed
    await nextTick();
    d.resolve({ items: ["c"], has_more: false });
    await pending;

    expect(p.items.value).toEqual(["z"]);
    expect(p.hasMore.value).toBe(false);
    expect(p.loadingMore.value).toBe(false);
  });

  it("keeps loaded pages when only other fields of the response change", async () => {
    const first = ref({ items: ["a"], has_more: true, show_empty: true });
    const p = useShowMore(first, async () => ({ items: ["b"], has_more: false }));
    await p.loadMore();
    first.value = { ...first.value, show_empty: false };
    await nextTick();
    expect(p.items.value).toEqual(["a", "b"]);
  });

  it("reports a failed page", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const first = ref<ItemsPage<string>>({ items: ["a"], has_more: true });
    const p = useShowMore(first, async () => {
      throw new Error("boom");
    });
    await p.loadMore();
    expect(p.error.value).toBe(true);
    expect(p.items.value).toEqual(["a"]);
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import CollectionCompare from "~/components/CollectionCompare.vue";
import type { CatalogEntry, CompareBucket, CompareResponse, CompareRow } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});
const AddToCollectionStub = defineComponent({
  name: "AddToCollection",
  props: { entry: { type: Object, required: true } },
  emits: ["added"],
  setup(props, { emit }) {
    return () =>
      h(
        "button",
        { "data-testid": "add-stub", onClick: () => emit("added", { id: "c9", name: "Shelf" }) },
        (props.entry as CatalogEntry).name,
      );
  },
});

const side = (quantity: number, collection_id: string, collection_name: string, conditions: string[] = []) => ({
  quantity,
  items: 1,
  conditions,
  collection_id,
  collection_name,
});

const entry = (id: string, name: string): CatalogEntry => ({
  id,
  category_id: "cat",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
  name,
  attributes: {},
  source: "seed",
  status: "approved",
});

const rowsByBucket: Record<CompareBucket, CompareRow[]> = {
  both: [
    {
      key: "entry:e1",
      name: "Tree House",
      identifier: "21318",
      catalog_entry_id: "e1",
      mine: side(1, "m1", "Lego Display Shelf"),
      theirs: side(3, "t1", "Lego Shelf", ["Built", "Used"]),
    },
  ],
  mine_only: [{ key: "entry:e2", name: "Millennium Falcon", catalog_entry_id: "e2", mine: side(1, "m1", "Lego Display Shelf"), theirs: null }],
  theirs_only: [{ key: "name:nasa apollo saturn v", name: "NASA Apollo Saturn V", mine: null, theirs: side(1, "t1", "Lego Shelf") }],
  neither: [
    { key: "entry:e3", name: "AT-AT", identifier: "75313", catalog_entry_id: "e3", mine: null, theirs: null, entry: entry("e3", "AT-AT") },
  ],
};

function response(over: Partial<CompareResponse> = {}, bucket: CompareBucket = "both"): CompareResponse {
  return {
    other: { handle: "noah", display_name: "Noah Williams", bio: "", avatar_color: "teal" },
    categories: [
      { slug: "coins", name: "Coins", public: true, mine_items: 0, their_items: 2 },
      { slug: "lego-sets", name: "Lego Sets", public: true, mine_items: 4, their_items: 4 },
    ],
    category: { slug: "lego-sets", name: "Lego Sets", public: true, catalog_entries: 8 },
    counts: { both: 2, mine_only: 2, theirs_only: 2, neither: 3 },
    bucket,
    items: rowsByBucket[bucket],
    has_more: false,
    ...over,
  };
}

let fetchCompare: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchCompare = vi.fn(async (_handle: string, p: { bucket: CompareBucket }) => response({}, p.bucket));
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useCompareActions = () => ({ fetchCompare });
});

function mountCompare(props: { category?: string; bucket?: CompareBucket } = { category: "lego-sets" }) {
  return mount(CollectionCompare, {
    props: { handle: "noah", bucket: "both", ...props },
    global: { stubs: { NuxtLink: NuxtLinkStub, AddToCollection: AddToCollectionStub, UserAvatar: true } },
    attachTo: document.body,
  });
}

const tid = (id: string) => `[data-testid="${id}"]`;

describe("CollectionCompare", () => {
  it("shows four tabs with counts and the open tab's items", async () => {
    const wrapper = mountCompare();
    await flushPromises();
    expect(fetchCompare).toHaveBeenCalledWith("noah", { category: "lego-sets", bucket: "both", limit: 50 });
    expect(wrapper.get("h1").text()).toBe("Compare with Noah Williams");

    const tabs = wrapper.findAll('[role="tab"]');
    expect(tabs.map((t) => t.text())).toEqual(["Both of you 2", "Only you 2", "Only Noah 2", "Neither of you 3"]);
    expect(wrapper.get(tid("compare-tab-both")).attributes("aria-selected")).toBe("true");

    const row = wrapper.get(tid("compare-row"));
    expect(row.text()).toContain("Tree House");
    expect(row.text()).toContain("21318");
    expect(row.get(tid("compare-row-mine")).text()).toMatch(/^You:\s*1 copy in Lego Display Shelf$/);
    expect(row.get(tid("compare-row-mine")).get("a").attributes("href")).toBe("/my/m1");
    expect(row.get(tid("compare-row-theirs")).text()).toMatch(/^Noah:\s*3 copies · Built, Used in Lego Shelf$/);
    expect(row.get(tid("compare-row-theirs")).get("a").attributes("href")).toBe("/u/noah/c/t1");
    expect(row.get(tid("compare-catalog-link")).attributes("href")).toBe("/categories/lego-sets/catalog/e1");
    // No raw keys or ids on screen.
    expect(wrapper.text()).not.toContain("entry:");
  });

  it("free-form rows have no catalog link", async () => {
    const wrapper = mountCompare({ category: "lego-sets", bucket: "theirs_only" });
    await flushPromises();
    const row = wrapper.get(tid("compare-row"));
    expect(row.text()).toContain("NASA Apollo Saturn V");
    expect(row.find(tid("compare-catalog-link")).exists()).toBe(false);
    expect(row.find(tid("compare-row-mine")).exists()).toBe(false);
  });

  it("asks the parent to change tab and category (the URL holds them)", async () => {
    const wrapper = mountCompare();
    await flushPromises();
    await wrapper.get(tid("compare-tab-neither")).trigger("click");
    expect(wrapper.emitted("update:bucket")).toEqual([["neither"]]);

    const select = wrapper.get(tid("compare-category"));
    expect(select.findAll("option").map((o) => o.text())).toEqual([
      "Coins (you 0 · Noah 2)",
      "Lego Sets (you 4 · Noah 4)",
    ]);
    await select.setValue("coins");
    expect(wrapper.emitted("update:category")).toEqual([["coins"]]);
  });

  it("opens the first category you both have when the URL names none", async () => {
    fetchCompare.mockResolvedValueOnce(response({ category: null, counts: null, items: [] }));
    const wrapper = mountCompare({});
    await flushPromises();
    expect(fetchCompare).toHaveBeenCalledWith("noah", { category: undefined, bucket: "both", limit: 50 });
    expect(wrapper.emitted("update:category")).toEqual([["lego-sets"]]);
  });

  it("reloads when the tab changes", async () => {
    const wrapper = mountCompare();
    await flushPromises();
    await wrapper.setProps({ bucket: "mine_only" });
    await flushPromises();
    expect(fetchCompare).toHaveBeenLastCalledWith("noah", { category: "lego-sets", bucket: "mine_only", limit: 50 });
    expect(wrapper.get(tid("compare-row")).text()).toContain("Millennium Falcon");
  });

  it("offers catalog items neither of you has, with an Add to my collection shortcut", async () => {
    const wrapper = mountCompare({ category: "lego-sets", bucket: "neither" });
    await flushPromises();
    const row = wrapper.get(tid("compare-row"));
    expect(row.text()).toContain("AT-AT");
    const toggle = row.get(tid("compare-add-toggle"));
    expect(toggle.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find(tid("add-stub")).exists()).toBe(false);

    await toggle.trigger("click");
    expect(toggle.attributes("aria-expanded")).toBe("true");
    const calls = fetchCompare.mock.calls.length;
    await wrapper.get(tid("add-stub")).trigger("click");
    await flushPromises();
    expect(wrapper.get(tid("compare-notice")).text()).toBe("Added AT-AT to Shelf.");
    expect(fetchCompare.mock.calls.length).toBe(calls + 1);
  });

  it("explains that neither needs a catalog", async () => {
    fetchCompare.mockResolvedValueOnce(
      response(
        {
          category: { slug: "stamps", name: "Stamps", public: true, catalog_entries: 0 },
          counts: { both: 0, mine_only: 1, theirs_only: 0, neither: 0 },
          items: [],
        },
        "neither",
      ),
    );
    const wrapper = mountCompare({ category: "stamps", bucket: "neither" });
    await flushPromises();
    expect(wrapper.get(tid("compare-empty")).text()).toContain("Stamps has no catalog yet");
  });

  it("says so when neither of you has anything yet", async () => {
    fetchCompare.mockResolvedValueOnce(response({ categories: [], category: null, counts: null, items: [] }));
    const wrapper = mountCompare({});
    await flushPromises();
    expect(wrapper.find(tid("compare-no-categories")).exists()).toBe(true);
    expect(wrapper.emitted("update:category")).toBeUndefined();
  });

  it("explains a person whose collections you can't see", async () => {
    fetchCompare.mockResolvedValueOnce("hidden");
    const wrapper = mountCompare();
    await flushPromises();
    expect(wrapper.get(tid("compare-hidden")).text()).toContain("You can’t compare with this person");
    expect(wrapper.find('[role="tab"]').exists()).toBe(false);
  });

  it("drops an unknown category from the URL", async () => {
    fetchCompare.mockResolvedValueOnce("unknown_category");
    const wrapper = mountCompare({ category: "spoons-cccccc" });
    await flushPromises();
    expect(wrapper.emitted("update:category")).toEqual([[undefined]]);
  });

  it("shows more, and drops a page that arrives after the tab changed", async () => {
    const page2: CompareRow = { ...rowsByBucket.both[0]!, key: "entry:e9", name: "Typewriter" };
    fetchCompare.mockResolvedValueOnce(response({ has_more: true }));
    const wrapper = mountCompare();
    await flushPromises();

    fetchCompare.mockResolvedValueOnce(response({ items: [page2], has_more: false }));
    await wrapper.get(tid("compare-show-more")).trigger("click");
    await flushPromises();
    expect(fetchCompare).toHaveBeenLastCalledWith("noah", { category: "lego-sets", bucket: "both", limit: 50, offset: 1 });
    expect(wrapper.findAll(tid("compare-row")).map((r) => r.find("p").text())).toEqual(["Tree House", "Typewriter"]);
    expect(wrapper.find(tid("compare-show-more")).exists()).toBe(false);

    // A slow "show more" for the old tab must not land on the new one.
    fetchCompare.mockResolvedValueOnce(response({ has_more: true }));
    await wrapper.setProps({ bucket: "both", category: "coins" });
    await flushPromises();
    let release: (v: CompareResponse) => void = () => {};
    fetchCompare.mockImplementationOnce(() => new Promise((r) => (release = r)));
    await wrapper.get(tid("compare-show-more")).trigger("click");
    fetchCompare.mockResolvedValueOnce(response({}, "mine_only"));
    await wrapper.setProps({ bucket: "mine_only" });
    await flushPromises();
    release(response({ items: [page2] }));
    await flushPromises();
    expect(wrapper.findAll(tid("compare-row")).map((r) => r.find("p").text())).toEqual(["Millennium Falcon"]);
  });
});

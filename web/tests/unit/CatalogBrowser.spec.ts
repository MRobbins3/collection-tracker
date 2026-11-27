import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { defineComponent, h, ref, type Ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import CatalogBrowser from "~/components/CatalogBrowser.vue";
import type { CatalogEntry, CatalogOwnershipResponse, CatalogPage, Category } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});
const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });

const category: Category = {
  id: "cat-1",
  slug: "lego-sets",
  name: "Lego Sets",
  catalog_identifier: "set_number",
  attribute_schema: { type: "object", properties: { set_number: { type: "string", title: "Set number" } } },
};

function entry(id: string, name: string, identifier?: string): CatalogEntry {
  return {
    id,
    category_id: "cat-1",
    category_slug: "lego-sets",
    category_name: "Lego Sets",
    name,
    identifier,
    attributes: {},
    source: "seed",
    status: "approved",
  };
}

let term: Readonly<Ref<string>>;
let data: Ref<CatalogPage>;
let pending: Ref<boolean>;
let error: Ref<Error | undefined>;
let ownership: Ref<CatalogOwnershipResponse>;
let fetchPage: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.useFakeTimers();
  data = ref({ entries: [], total: 0, has_more: false });
  pending = ref(false);
  error = ref(undefined);
  ownership = ref({ owned: [] });
  fetchPage = vi.fn();
  // @ts-expect-error vitest globals standing in for Nuxt auto-imports
  globalThis.useCatalogPage = (_slug: string, q: Readonly<Ref<string>>) => {
    term = q;
    return { data, pending, error, refresh: vi.fn() };
  };
  // @ts-expect-error vitest global standing in for a Nuxt auto-import
  globalThis.useCatalogOwnership = () => ({ data: ownership });
  // @ts-expect-error vitest global standing in for a Nuxt auto-import
  globalThis.useCatalogActions = () => ({ fetchPage });
});

afterEach(() => {
  vi.useRealTimers();
});

function mountBrowser() {
  return mount(CatalogBrowser, {
    props: { category },
    global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub } },
  });
}

describe("CatalogBrowser", () => {
  it("lists entries with their labelled identifier and a link to each", async () => {
    data.value = { entries: [entry("e1", "Millennium Falcon", "75192")], total: 1, has_more: false };
    const wrapper = mountBrowser();
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="catalog-entry"]');
    expect(rows).toHaveLength(1);
    expect(rows[0]!.text()).toContain("Millennium Falcon");
    expect(rows[0]!.text()).toContain("Set number 75192");
    expect(rows[0]!.text()).not.toContain("set_number");
    expect(rows[0]!.find("a").attributes("href")).toBe("/categories/lego-sets/catalog/e1");
    expect(wrapper.get('[data-testid="catalog-status"]').text()).toBe("1 entry");
  });

  it("marks entries already in your collection", async () => {
    data.value = { entries: [entry("e1", "Falcon"), entry("e2", "AT-AT")], total: 2, has_more: false };
    ownership.value = { owned: [{ catalog_entry_id: "e2", item_id: "i", collection_id: "c", collection_name: "Shelf" }] };
    const wrapper = mountBrowser();
    await flushPromises();
    const rows = wrapper.findAll('[data-testid="catalog-entry"]');
    expect(rows[0]!.find('[data-testid="catalog-entry-owned"]').exists()).toBe(false);
    expect(rows[1]!.find('[data-testid="catalog-entry-owned"]').text()).toContain("In your collection");
  });

  it("searches with the normalized, debounced text", async () => {
    const wrapper = mountBrowser();
    await wrapper.get('[data-testid="catalog-search"]').setValue("  millennium   falcon ");
    expect(term.value).toBe("");
    vi.advanceTimersByTime(300);
    expect(term.value).toBe("millennium falcon");
    await flushPromises();
    expect(wrapper.get('[data-testid="catalog-empty"]').text()).toContain("Nothing in the catalog matches “millennium falcon”");
  });

  it("shows an empty state when the category has no catalog yet", async () => {
    const wrapper = mountBrowser();
    await flushPromises();
    expect(wrapper.get('[data-testid="catalog-empty"]').text()).toContain("No Lego Sets in the catalog yet");
  });

  it("loads more pages from the current offset", async () => {
    data.value = { entries: [entry("e1", "A"), entry("e2", "B")], total: 3, has_more: true };
    fetchPage.mockResolvedValue({ entries: [entry("e3", "C")], total: 3, has_more: false });
    const wrapper = mountBrowser();
    await flushPromises();

    await wrapper.get('[data-testid="catalog-show-more"]').trigger("click");
    await flushPromises();
    expect(fetchPage).toHaveBeenCalledWith("lego-sets", "", 2);
    expect(wrapper.findAll('[data-testid="catalog-entry"]')).toHaveLength(3);
    expect(wrapper.find('[data-testid="catalog-show-more"]').exists()).toBe(false);
  });

  it("drops a page that arrives after the search changed", async () => {
    data.value = { entries: [entry("e1", "A")], total: 2, has_more: true };
    let resolve!: (p: CatalogPage) => void;
    fetchPage.mockReturnValue(new Promise<CatalogPage>((r) => (resolve = r)));
    const wrapper = mountBrowser();
    await flushPromises();

    await wrapper.get('[data-testid="catalog-show-more"]').trigger("click");
    data.value = { entries: [entry("e9", "New search")], total: 1, has_more: false };
    await flushPromises();
    resolve({ entries: [entry("e2", "Stale")], total: 2, has_more: false });
    await flushPromises();

    expect(wrapper.text()).not.toContain("Stale");
    expect(wrapper.findAll('[data-testid="catalog-entry"]')).toHaveLength(1);
  });

  it("shows an error with a retry", async () => {
    error.value = new Error("boom");
    const wrapper = mountBrowser();
    await flushPromises();
    expect(wrapper.get('[data-testid="catalog-error"]').text()).toContain("Couldn’t load the catalog");
  });
});

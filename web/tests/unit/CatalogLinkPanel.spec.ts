import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import CatalogLinkPanel from "~/components/CatalogLinkPanel.vue";
import type { CatalogSuggestion, Item } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});
const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });

const item: Item = {
  id: "item-1",
  collection_id: "col-1",
  name: "Falcon",
  quantity: 1,
  attributes: { set_number: "75192" },
  created_at: "",
  updated_at: "",
};

const suggestion: CatalogSuggestion = {
  id: "entry-1",
  category_id: "cat-1",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
  name: "Millennium Falcon",
  identifier: "75192",
  attributes: {},
  source: "seed",
  status: "approved",
  match: "identifier",
};

let actions: {
  suggestions: ReturnType<typeof vi.fn>;
  link: ReturnType<typeof vi.fn>;
  unlink: ReturnType<typeof vi.fn>;
};

beforeEach(() => {
  actions = {
    suggestions: vi.fn(async () => ({ suggestions: [suggestion] })),
    link: vi.fn(async () => ({ ...item, catalog_entry_id: "entry-1" })),
    unlink: vi.fn(async () => ({ ...item, catalog_entry_id: undefined })),
  };
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useCatalogActions = () => actions;
  submitEntry = vi.fn(async () => ({ outcome: "submitted", item_linked: false }));
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useSubmissionActions = () => ({ submitEntry, proposeCorrection: vi.fn() });
});

let submitEntry: ReturnType<typeof vi.fn>;

function mountPanel(it: Item = item) {
  return mount(CatalogLinkPanel, {
    props: { item: it, categorySlug: "lego-sets", identifierTitle: "Set number" },
    global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub } },
  });
}

describe("CatalogLinkPanel", () => {
  it("suggests matches only when asked, explaining why each matched", async () => {
    const wrapper = mountPanel();
    expect(actions.suggestions).not.toHaveBeenCalled();

    await wrapper.get("button").trigger("click");
    await flushPromises();
    expect(actions.suggestions).toHaveBeenCalledWith("col-1", "item-1");
    const row = wrapper.get('[data-testid="catalog-suggestion"]');
    expect(row.text()).toContain("Millennium Falcon");
    expect(row.text()).toContain("Set number 75192");
    expect(row.text()).toContain("Same set number");
    expect(actions.link).not.toHaveBeenCalled();
  });

  it("links the picked suggestion and reports the updated item", async () => {
    const wrapper = mountPanel();
    await wrapper.get("button").trigger("click");
    await flushPromises();
    await wrapper.get('button[aria-label="Link to Millennium Falcon"]').trigger("click");
    await flushPromises();
    expect(actions.link).toHaveBeenCalledWith("col-1", "item-1", "entry-1");
    expect(wrapper.emitted("changed")?.[0]?.[0]).toMatchObject({ catalog_entry_id: "entry-1" });
  });

  it("says so when nothing matches", async () => {
    actions.suggestions.mockResolvedValue({ suggestions: [] });
    const wrapper = mountPanel();
    await wrapper.get("button").trigger("click");
    await flushPromises();
    expect(wrapper.get('[data-testid="catalog-suggestions-empty"]').text()).toContain("No likely matches");
  });

  it("shows a linked item's catalog link and unlinks it", async () => {
    const wrapper = mountPanel({ ...item, catalog_entry_id: "entry-1" });
    expect(wrapper.get('[data-testid="catalog-linked"]').attributes("href")).toBe("/categories/lego-sets/catalog/entry-1");
    await wrapper.get("button").trigger("click");
    await flushPromises();
    expect(actions.unlink).toHaveBeenCalledWith("col-1", "item-1");
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  it("shows an error when the catalog can't be searched", async () => {
    actions.suggestions.mockRejectedValue(new Error("boom"));
    vi.spyOn(console, "error").mockImplementation(() => {});
    const wrapper = mountPanel();
    await wrapper.get("button").trigger("click");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("Couldn’t search the catalog");
  });

  it("suggests a free-form item for the catalog, prefilled, with the item to link", async () => {
    const wrapper = mountPanel();
    await wrapper.get('[data-testid="catalog-suggest"]').trigger("click");
    const name = wrapper.get('[data-testid="suggest-entry-name"]').element as HTMLInputElement;
    expect(name.value).toBe("Falcon");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(submitEntry).toHaveBeenCalledWith(
      expect.objectContaining({ category_slug: "lego-sets", name: "Falcon", item_id: "item-1" }),
    );
    expect(wrapper.get('[data-testid="suggest-entry-result"]').text()).toContain("An admin will review");
    expect(wrapper.emitted("changed")).toBeUndefined();
  });

  it("reports the item as linked when the suggestion was already in the catalog", async () => {
    submitEntry.mockResolvedValue({ outcome: "already_in_catalog", item_linked: true, entry: { ...suggestion } });
    const wrapper = mountPanel();
    await wrapper.get('[data-testid="catalog-suggest"]').trigger("click");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.emitted("changed")?.[0]?.[0]).toMatchObject({ catalog_entry_id: "entry-1" });
  });
});

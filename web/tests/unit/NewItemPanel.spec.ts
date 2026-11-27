import { describe, it, expect, vi, beforeEach } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import NewItemPanel from "~/components/NewItemPanel.vue";
import type { AttributeSchema, CatalogEntry } from "~/types/api";

const schema: AttributeSchema = {
  type: "object",
  properties: {
    set_number: { type: "string", title: "Set number" },
  },
};

let searchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  // Default: catalog returns empty, reflecting MVP reality.
  searchMock = vi.fn(async (): Promise<{ entries: CatalogEntry[] }> => ({ entries: [] }));
  // @ts-expect-error test global
  globalThis.useCatalogSearch = () => ({ search: searchMock });
});

describe("NewItemPanel", () => {
  it("does not call the catalog when the query is empty", async () => {
    mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await flushPromises();
    expect(searchMock).not.toHaveBeenCalled();
  });

  it("shows the empty-catalog hint when a search returns nothing", async () => {
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await wrapper.get('[data-testid="new-item-search"]').setValue("falcon");
    // Debounce is 200ms — wait it out deterministically.
    await new Promise((r) => setTimeout(r, 220));
    await flushPromises();
    expect(searchMock).toHaveBeenCalledWith("lego-sets", "falcon");
    expect(wrapper.find('[data-testid="new-item-empty-catalog"]').exists()).toBe(true);
  });

  it("clicking Add manually opens the item form with query prefilled", async () => {
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await wrapper.get('[data-testid="new-item-search"]').setValue("Millennium Falcon");
    await wrapper.get('[data-testid="new-item-manual"]').trigger("click");
    await flushPromises();

    // Panel swaps to the ItemForm — confirm it's there and the name field has
    // the query we typed.
    const name = wrapper.find('[data-testid="item-form-name"]');
    expect(name.exists()).toBe(true);
    expect((name.element as HTMLInputElement).value).toBe("Millennium Falcon");
  });

  it("emits create when the manual form is submitted", async () => {
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await wrapper.get('[data-testid="new-item-manual"]').trigger("click");
    await flushPromises();

    await wrapper.get('[data-testid="item-form-name"]').setValue("Manual Entry");
    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();

    const emitted = wrapper.emitted("create");
    expect(emitted).toBeTruthy();
    expect(emitted?.[0]?.[0]).toMatchObject({ name: "Manual Entry", quantity: 1 });
  });

  const falcon: CatalogEntry = {
    id: "entry-1",
    category_id: "cat-1",
    category_slug: "lego-sets",
    category_name: "Lego Sets",
    name: "Millennium Falcon",
    identifier: "75192",
    attributes: { set_number: "75192" },
    source: "seed",
    status: "approved",
  };

  async function searchFor(wrapper: ReturnType<typeof mount>, text: string) {
    await wrapper.get('[data-testid="new-item-search"]').setValue(text);
    await new Promise((r) => setTimeout(r, 220));
    await flushPromises();
  }

  it("lists catalog matches with the labelled identifier", async () => {
    searchMock.mockResolvedValue({ entries: [falcon] });
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema, identifierTitle: "Set number" } });
    await searchFor(wrapper, "  falcon ");
    expect(searchMock).toHaveBeenCalledWith("lego-sets", "falcon");
    const result = wrapper.get('[data-testid="new-item-result"]');
    expect(result.text()).toContain("Millennium Falcon");
    expect(result.text()).toContain("Set number 75192");
  });

  it("picking a match prefills the form and links the new item to it", async () => {
    searchMock.mockResolvedValue({ entries: [falcon] });
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await searchFor(wrapper, "falcon");
    await wrapper.get('[data-testid="new-item-result"]').trigger("click");
    await flushPromises();

    expect(wrapper.get('[data-testid="new-item-linked"]').text()).toContain("Millennium Falcon");
    const name = wrapper.get('[data-testid="item-form-name"]').element as HTMLInputElement;
    expect(name.value).toBe("Millennium Falcon");

    // Personalize the name; the link stays.
    await wrapper.get('[data-testid="item-form-name"]').setValue("Falcon (signed box)");
    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();
    expect(wrapper.emitted("create")?.[0]?.[0]).toMatchObject({
      name: "Falcon (signed box)",
      attributes: { set_number: "75192" },
      catalog_entry_id: "entry-1",
    });
  });

  it("“Don’t link” keeps the prefilled details but adds a free-form item", async () => {
    searchMock.mockResolvedValue({ entries: [falcon] });
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await searchFor(wrapper, "falcon");
    await wrapper.get('[data-testid="new-item-result"]').trigger("click");
    await wrapper.get('[data-testid="new-item-unlink"]').trigger("click");
    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();

    const payload = wrapper.emitted("create")?.[0]?.[0] as Record<string, unknown>;
    expect(payload).toMatchObject({ name: "Millennium Falcon" });
    expect(payload.catalog_entry_id).toBeUndefined();
  });

  it("ignores a slow response for an older search", async () => {
    let resolveOld!: (v: { entries: CatalogEntry[] }) => void;
    searchMock
      .mockReturnValueOnce(new Promise((r) => (resolveOld = r)))
      .mockResolvedValueOnce({ entries: [] });
    const wrapper = mount(NewItemPanel, { props: { categorySlug: "lego-sets", schema } });
    await searchFor(wrapper, "fal");
    await searchFor(wrapper, "zzz");
    resolveOld({ entries: [falcon] });
    await flushPromises();
    expect(wrapper.find('[data-testid="new-item-results"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="new-item-empty-catalog"]').exists()).toBe(true);
  });
});

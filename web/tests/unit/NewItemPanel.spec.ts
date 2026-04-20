import { describe, it, expect, vi, beforeEach } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import NewItemPanel from "~/components/NewItemPanel.vue";
import type { AttributeSchema } from "~/types/api";

const schema: AttributeSchema = {
  type: "object",
  properties: {
    set_number: { type: "string", title: "Set number" },
  },
};

let searchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  // Default: catalog returns empty, reflecting MVP reality.
  searchMock = vi.fn(async () => ({ entries: [] }));
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
    expect(emitted?.[0][0]).toMatchObject({ name: "Manual Entry", quantity: 1 });
  });
});

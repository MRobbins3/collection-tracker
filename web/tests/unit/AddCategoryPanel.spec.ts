import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import AddCategoryPanel from "~/components/AddCategoryPanel.vue";
import type { Category, OverviewCategory } from "~/types/api";

const IconStub = defineComponent({
  name: "Icon",
  props: { name: { type: String, required: true } },
  setup() {
    return () => h("span");
  },
});
const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});

const all: Category[] = [
  { id: "1", slug: "coins", name: "Coins", description: "Money of the world", attribute_schema: {} },
  { id: "2", slug: "lego-sets", name: "Lego Sets", attribute_schema: {} },
  { id: "3", slug: "stamps", name: "Stamps", attribute_schema: {} },
];
const overview: OverviewCategory[] = [
  { slug: "lego-sets", name: "Lego Sets", item_count: 2, collection_count: 1, pinned: false },
  { slug: "stamps", name: "Stamps", item_count: 0, collection_count: 0, pinned: true },
];

function mountPanel(props: Record<string, unknown> = {}) {
  return mount(AddCategoryPanel, {
    props: { categories: all, overview, busySlug: null, error: null, ...props },
    global: { stubs: { Icon: IconStub, NuxtLink: NuxtLinkStub } },
  });
}

describe("AddCategoryPanel", () => {
  it("lists every category with its state in your collection", () => {
    const wrapper = mountPanel();
    expect(wrapper.get("h2").text()).toBe("Add a category");
    const rows = wrapper.findAll('[data-testid="add-category-row"]');
    expect(rows.map((r) => r.text())).toEqual([
      expect.stringContaining("Coins"),
      expect.stringContaining("Lego Sets"),
      expect.stringContaining("Stamps"),
    ]);
    expect(rows[0]!.text()).toContain("Money of the world");
    expect(rows[0]!.get("button").text()).toBe("Add");
    expect(rows[1]!.text()).toContain("In my collection");
    expect(rows[2]!.text()).toContain("Added");
  });

  it("emits pin and unpin with the category", async () => {
    const wrapper = mountPanel();
    await wrapper.get('[data-testid="pin-category-coins"]').trigger("click");
    await wrapper.get('[data-testid="unpin-category-stamps"]').trigger("click");
    expect(wrapper.emitted("pin")).toEqual([["coins"]]);
    expect(wrapper.emitted("unpin")).toEqual([["stamps"]]);
  });

  it("disables the row being saved and shows errors", () => {
    const wrapper = mountPanel({ busySlug: "coins", error: "Couldn’t add that category." });
    expect(wrapper.get('[data-testid="pin-category-coins"]').attributes("disabled")).toBeDefined();
    expect(wrapper.get('[role="alert"]').text()).toContain("Couldn’t add");
  });

  it("closes", async () => {
    const wrapper = mountPanel();
    await wrapper.get('[data-testid="add-category-close"]').trigger("click");
    expect(wrapper.emitted("close")).toHaveLength(1);
  });
});

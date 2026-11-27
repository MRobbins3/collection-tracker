import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import OverviewControls from "~/components/OverviewControls.vue";
import type { OverviewCategory } from "~/types/api";

const IconStub = defineComponent({
  name: "Icon",
  props: { name: { type: String, required: true } },
  setup() {
    return () => h("span");
  },
});

const categories: OverviewCategory[] = [
  { slug: "lego-sets", name: "Lego Sets", item_count: 4, collection_count: 1, pinned: false },
  { slug: "coins", name: "Coins", item_count: 0, collection_count: 0, pinned: true },
];

function mountControls(props: Partial<InstanceType<typeof OverviewControls>["$props"]> = {}) {
  return mount(OverviewControls, {
    props: {
      view: "collections",
      categories,
      selected: [],
      showEmpty: true,
      ...props,
    },
    global: { stubs: { Icon: IconStub } },
  });
}

describe("OverviewControls", () => {
  it("switches between the collections and items views", async () => {
    const wrapper = mountControls();
    const group = wrapper.get('[role="group"][aria-label="View"]');
    const [collections, items] = group.findAll("button");
    expect(collections!.text()).toBe("Collections");
    expect(collections!.attributes("aria-pressed")).toBe("true");
    expect(items!.attributes("aria-pressed")).toBe("false");

    await items!.trigger("click");
    expect(wrapper.emitted("update:view")).toEqual([["items"]]);
  });

  it("shows a chip per category with its count and an All chip", () => {
    const wrapper = mountControls();
    const chips = wrapper.get('[role="group"][aria-label="Filter by category"]').findAll("button");
    expect(chips.map((c) => c.text())).toEqual(["All", "Lego Sets 4 items", "Coins 0 items"]);
    expect(chips[0]!.attributes("aria-pressed")).toBe("true");
    expect(wrapper.text()).not.toContain("lego-sets");
  });

  it("toggles a category in and out of the filter", async () => {
    const wrapper = mountControls({ selected: ["coins"] });
    const chip = (name: string) => wrapper.get(`[data-testid="category-chip-${name}"]`);
    expect(chip("coins").attributes("aria-pressed")).toBe("true");
    expect(wrapper.get('[data-testid="category-chip-all"]').attributes("aria-pressed")).toBe("false");

    await chip("lego-sets").trigger("click");
    await chip("coins").trigger("click");
    expect(wrapper.emitted("update:selected")).toEqual([[["coins", "lego-sets"]], [[]]]);

    await wrapper.get('[data-testid="category-chip-all"]').trigger("click");
    expect(wrapper.emitted("update:selected")![2]).toEqual([[]]);
  });

  it("has a labelled switch for showing empty categories", async () => {
    const wrapper = mountControls({ showEmpty: false });
    const sw = wrapper.get('[role="switch"]');
    expect(sw.attributes("aria-checked")).toBe("false");
    expect(sw.text()).toContain("Show empty categories");
    await sw.trigger("click");
    expect(wrapper.emitted("update:showEmpty")).toEqual([[true]]);
  });

  it("offers adding a category", async () => {
    const wrapper = mountControls();
    await wrapper.get('[data-testid="add-category-button"]').trigger("click");
    expect(wrapper.emitted("addCategory")).toHaveLength(1);
  });

  it("hides the chip row when there are no categories", () => {
    const wrapper = mountControls({ categories: [] });
    expect(wrapper.find('[aria-label="Filter by category"]').exists()).toBe(false);
  });
});

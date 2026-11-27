import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import CategoryGroups from "~/components/CategoryGroups.vue";
import type { Collection, OverviewCategory } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});

const coll = (id: string, slug: string, name: string, item_count: number): Collection => ({
  id,
  category_id: `cat-${slug}`,
  category_slug: slug,
  category_name: slug,
  name,
  item_count,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
});

const categories: OverviewCategory[] = [
  { slug: "coins", name: "Coins", item_count: 0, collection_count: 0, pinned: true },
  { slug: "lego-sets", name: "Lego Sets", item_count: 5, collection_count: 2, pinned: false },
  { slug: "stamps", name: "Stamps", item_count: 0, collection_count: 1, pinned: false },
];
const collections = [
  coll("c1", "lego-sets", "Lego Display Shelf", 4),
  coll("c2", "lego-sets", "Lego Bin", 1),
  coll("c3", "stamps", "Stamp Album", 0),
  coll("c4", "vinyl-records", "Records", 2), // its category is filtered out
];

function mountGroups(props: Record<string, unknown> = {}) {
  return mount(CategoryGroups, {
    props: { categories, collections, busySlug: null, ...props },
    global: { stubs: { NuxtLink: NuxtLinkStub } },
  });
}

describe("CategoryGroups", () => {
  it("groups collections under their category headings, in the given order", () => {
    const wrapper = mountGroups();
    const groups = wrapper.findAll("section");
    expect(groups.map((g) => g.get("h2").text())).toEqual(["Coins", "Lego Sets", "Stamps"]);
    expect(groups[1]!.text()).toContain("5 items");
    expect(groups[1]!.findAll('[data-testid^="my-collection-"]').map((l) => l.get("h3").text())).toEqual([
      "Lego Display Shelf",
      "Lego Bin",
    ]);
    expect(groups[1]!.text()).toContain("4 items");
    expect(groups[1]!.get("a").attributes("href")).toBe("/my/c1");
    expect(wrapper.text()).not.toContain("Records");
  });

  it("an empty collection's category says nothing is added yet", () => {
    const stamps = mountGroups().get('[data-testid="category-group-stamps"]');
    expect(stamps.text()).toContain("Nothing added yet");
    expect(stamps.text()).toContain("Stamp Album");
    expect(stamps.find('[data-testid="start-collection-stamps"]').exists()).toBe(false);
  });

  it("an added category with no collections offers to start one or remove it", async () => {
    const wrapper = mountGroups();
    const coins = wrapper.get('[data-testid="category-group-coins"]');
    expect(coins.text()).toContain("Nothing added yet");
    await coins.get('[data-testid="start-collection-coins"]').trigger("click");
    await coins.get('[data-testid="remove-category-coins"]').trigger("click");
    expect(wrapper.emitted("startCollection")).toEqual([["coins", "Coins"]]);
    expect(wrapper.emitted("unpin")).toEqual([["coins"]]);
    expect(coins.get('[data-testid="remove-category-coins"]').attributes("aria-label")).toBe(
      "Remove Coins from my collection",
    );
  });

  it("disables a group's buttons while it is saving", () => {
    const coins = mountGroups({ busySlug: "coins" }).get('[data-testid="category-group-coins"]');
    expect(coins.get('[data-testid="start-collection-coins"]').attributes("disabled")).toBeDefined();
    expect(coins.get('[data-testid="remove-category-coins"]').attributes("disabled")).toBeDefined();
  });

  it("emits delete with the collection's id and name", async () => {
    const wrapper = mountGroups();
    await wrapper.get('[data-testid="delete-collection-c2"]').trigger("click");
    expect(wrapper.emitted("deleteCollection")).toEqual([["c2", "Lego Bin"]]);
  });
});

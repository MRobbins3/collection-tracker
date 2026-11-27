import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import OverviewItemList from "~/components/OverviewItemList.vue";
import type { OverviewItem } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});

const items: OverviewItem[] = [
  {
    id: "i1",
    name: "Rumours",
    quantity: 1,
    collection_id: "col-2",
    collection_name: "Classic Rock Vinyl",
    category_slug: "vinyl-records",
    category_name: "Vinyl Records",
    created_at: "2026-06-21T12:00:00Z",
  },
  {
    id: "i2",
    name: "Kennedy Half Dollar",
    quantity: 3,
    collection_id: "col-3",
    collection_name: "US Coins",
    category_slug: "coins",
    category_name: "Coins",
    created_at: "2026-04-02T12:00:00Z",
  },
];

function mountList(props: Record<string, unknown> = {}) {
  return mount(OverviewItemList, {
    props: {
      items,
      total: 2,
      hasMore: false,
      sort: "newest",
      pending: false,
      loadingMore: false,
      error: false,
      filtered: false,
      ...props,
    },
    global: { stubs: { NuxtLink: NuxtLinkStub } },
  });
}

describe("OverviewItemList", () => {
  it("has a labelled sort control with human labels", async () => {
    const wrapper = mountList();
    const select = wrapper.get("select");
    const id = select.attributes("id");
    expect(wrapper.get(`label[for="${id}"]`).text()).toBe("Sort by");
    expect(select.findAll("option").map((o) => o.text())).toEqual([
      "Newest first",
      "Oldest first",
      "Name (A–Z)",
      "Name (Z–A)",
      "Category",
    ]);
    await select.setValue("name_asc");
    expect(wrapper.emitted("update:sort")).toEqual([["name_asc"]]);
  });

  it("lists items with where they live and when they were added", () => {
    const wrapper = mountList();
    expect(wrapper.get('[data-testid="overview-item-count"]').text()).toBe("2 items");
    const rows = wrapper.findAll('[data-testid="overview-item"]');
    expect(rows).toHaveLength(2);
    expect(rows[0]!.get("a").attributes("href")).toBe("/my/col-2");
    expect(rows[0]!.text()).toContain("Rumours");
    expect(rows[0]!.text()).toContain("Classic Rock Vinyl · Vinyl Records");
    expect(rows[0]!.text()).toContain("Added");
    expect(rows[0]!.get("time").attributes("datetime")).toBe("2026-06-21T12:00:00Z");
    expect(rows[0]!.text()).not.toContain("×1");
    expect(rows[1]!.text()).toContain("×3");
    expect(wrapper.text()).not.toContain("vinyl-records");
  });

  it("loads more when there are more items", async () => {
    const wrapper = mountList({ hasMore: true, total: 120 });
    expect(wrapper.get('[data-testid="overview-item-count"]').text()).toBe("Showing 2 of 120 items");
    await wrapper.get('[data-testid="overview-load-more"]').trigger("click");
    expect(wrapper.emitted("loadMore")).toHaveLength(1);

    const busy = mountList({ hasMore: true, loadingMore: true });
    const btn = busy.get('[data-testid="overview-load-more"]');
    expect(btn.attributes("disabled")).toBeDefined();
    expect(btn.text()).toContain("Loading");
  });

  it("shows loading, error, and empty states", async () => {
    expect(mountList({ items: [], total: 0, pending: true }).find('[data-testid="overview-items-loading"]').exists()).toBe(true);

    const err = mountList({ error: true });
    expect(err.get('[role="alert"]').text()).toContain("Couldn’t load your items");
    await err.get('[role="alert"] button').trigger("click");
    expect(err.emitted("retry")).toHaveLength(1);

    expect(mountList({ items: [], total: 0 }).get('[data-testid="overview-items-empty"]').text()).toContain(
      "haven’t added any items",
    );
    expect(
      mountList({ items: [], total: 0, filtered: true }).get('[data-testid="overview-items-empty"]').text(),
    ).toContain("No items in the selected categories");
  });
});

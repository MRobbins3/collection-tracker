import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import ItemCard from "~/components/ItemCard.vue";
import type { Item } from "~/types/api";

const item: Item = {
  id: "i1",
  collection_id: "c1",
  name: "Millennium Falcon",
  quantity: 1,
  condition: "New in box",
  attributes: { set_number: "75192" },
  created_at: "2026-11-27T00:00:00Z",
  updated_at: "2026-11-27T00:00:00Z",
};

describe("ItemCard", () => {
  it("offers Edit and Delete to the owner", () => {
    const wrapper = mount(ItemCard, { props: { item } });
    expect(wrapper.find('[data-testid="item-i1-edit"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="item-i1-delete"]').exists()).toBe(true);
  });

  it("is read-only on someone else's profile, with human attribute labels", () => {
    const wrapper = mount(ItemCard, { props: { item, readonly: true } });
    expect(wrapper.find("button").exists()).toBe(false);
    expect(wrapper.text()).toContain("Set number");
    expect(wrapper.text()).not.toContain("set_number");
  });
});

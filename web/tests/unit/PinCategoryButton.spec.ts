import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import PinCategoryButton from "~/components/PinCategoryButton.vue";

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

function mountButton(state: "none" | "pinned" | "collecting", busy = false) {
  return mount(PinCategoryButton, {
    props: { slug: "coins", name: "Coins", state, busy },
    global: { stubs: { Icon: IconStub, NuxtLink: NuxtLinkStub } },
  });
}

describe("PinCategoryButton", () => {
  it("offers to add a category that isn't in your collection", async () => {
    const wrapper = mountButton("none");
    const btn = wrapper.get("button");
    expect(btn.text()).toBe("Add");
    expect(btn.attributes("aria-label")).toBe("Add Coins to my collection");
    await btn.trigger("click");
    expect(wrapper.emitted("add")).toHaveLength(1);
  });

  it("shows an added category with a way to remove it", async () => {
    const wrapper = mountButton("pinned");
    expect(wrapper.text()).toContain("Added");
    const btn = wrapper.get("button");
    expect(btn.attributes("aria-label")).toBe("Remove Coins from my collection");
    await btn.trigger("click");
    expect(wrapper.emitted("remove")).toHaveLength(1);
  });

  it("links to the overview for a category you already collect", () => {
    const wrapper = mountButton("collecting");
    expect(wrapper.find("button").exists()).toBe(false);
    const link = wrapper.get("a");
    expect(link.attributes("href")).toBe("/my?category=coins");
    expect(link.text()).toBe("In my collection");
  });

  it("disables the button while saving", () => {
    const wrapper = mountButton("none", true);
    expect(wrapper.get("button").attributes("disabled")).toBeDefined();
  });
});

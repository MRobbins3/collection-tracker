import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import CategoryCard from "~/components/CategoryCard.vue";
import type { Category } from "~/types/api";

// NuxtLink isn't available in the Vitest environment; stub it to a plain <a>
// so we can still assert the href the card links to.
const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () =>
      h(
        "a",
        { href: typeof props.to === "string" ? props.to : "#", class: "nuxt-link-stub" },
        slots.default ? slots.default() : [],
      );
  },
});

const sample = (overrides: Partial<Category> = {}): Category => ({
  id: "id-1",
  slug: "lego-sets",
  name: "Lego Sets",
  description: "Official Lego sets.",
  attribute_schema: {},
  ...overrides,
});

describe("CategoryCard", () => {
  it("renders the category name and description", () => {
    const wrapper = mount(CategoryCard, {
      props: { category: sample() },
      global: { stubs: { NuxtLink: NuxtLinkStub } },
    });
    expect(wrapper.text()).toContain("Lego Sets");
    expect(wrapper.text()).toContain("Official Lego sets.");
  });

  it("links to the category detail page by slug", () => {
    const wrapper = mount(CategoryCard, {
      props: { category: sample({ slug: "funko-pops" }) },
      global: { stubs: { NuxtLink: NuxtLinkStub } },
    });
    const a = wrapper.find("a");
    expect(a.attributes("href")).toBe("/categories/funko-pops");
  });

  it("omits the description paragraph when none is provided", () => {
    const wrapper = mount(CategoryCard, {
      props: { category: sample({ description: undefined }) },
      global: { stubs: { NuxtLink: NuxtLinkStub } },
    });
    expect(wrapper.find("p").exists()).toBe(false);
  });

  it("exposes a stable testid keyed by slug for Playwright selectors", () => {
    const wrapper = mount(CategoryCard, {
      props: { category: sample({ slug: "vinyl-records" }) },
      global: { stubs: { NuxtLink: NuxtLinkStub } },
    });
    expect(wrapper.attributes("data-testid")).toBe("category-card-vinyl-records");
  });
});

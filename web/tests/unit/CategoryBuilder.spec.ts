import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import CategoryBuilder from "~/components/CategoryBuilder.vue";
import type { Category, CategoryDraft, LibraryField } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: String(props.to) }, slots.default?.());
  },
});

const library: LibraryField[] = [
  {
    id: "string:theme", key: "theme", title: "Theme", type: "string", options: ["Star Wars", "Presidents"],
    sources: [
      { slug: "lego-sets", name: "Lego Sets", options: ["Star Wars"] },
      { slug: "coins", name: "Coins", options: ["Presidents"] },
    ],
  },
  {
    id: "integer:year", key: "year", title: "Year", type: "integer", options: [],
    sources: [{ slug: "coins", name: "Coins" }, { slug: "vinyl-records", name: "Vinyl Records" }],
  },
  {
    id: "string:artist", key: "artist", title: "Artist", type: "string", options: [],
    sources: [{ slug: "vinyl-records", name: "Vinyl Records" }],
  },
];

function mountBuilder(props: Record<string, unknown> = {}) {
  return mount(CategoryBuilder, {
    props: { library, ...props },
    global: { stubs: { NuxtLink: NuxtLinkStub, Icon: true } },
  });
}

describe("CategoryBuilder", () => {
  it("lists library fields grouped by the category they come from, and filters them", async () => {
    const w = mountBuilder();
    expect(w.find('[data-testid="builder-group-lego-sets"]').text()).toContain("Theme");
    expect(w.find('[data-testid="builder-group-vinyl-records"]').text()).toContain("Year");
    expect(w.find('[data-testid="builder-group-coins"]').text()).toContain("Year");
    await w.get('[data-testid="builder-field-search"]').setValue("artist");
    expect(w.find('[data-testid="builder-group-lego-sets"]').exists()).toBe(false);
    expect(w.find('[data-testid="builder-group-vinyl-records"]').text()).toContain("Artist");
  });

  it("builds a draft from picked fields with a new option, and previews the item form", async () => {
    const w = mountBuilder();
    await w.get('[data-testid="builder-name"]').setValue("brick sets");
    await w.get('[data-testid="builder-name-suggestion"]').trigger("click");
    expect((w.get('[data-testid="builder-name"]').element as HTMLInputElement).value).toBe("Brick Sets");

    await w.get('[data-testid="builder-add-lego-sets-theme"]').trigger("click");
    await w.get('[data-testid="builder-add-vinyl-records-year"]').trigger("click");
    // The same field from another category is already added.
    expect(w.get('[data-testid="builder-add-coins-year"]').attributes("disabled")).toBeDefined();

    await w.get('[data-testid="builder-option-input-theme"]').setValue("star wars");
    await w.get('[data-testid="builder-option-add-theme"]').trigger("click");
    expect(w.get('[data-testid="builder-option-feedback-theme"]').text()).toContain("“Star Wars” is already in the list");

    await w.get('[data-testid="builder-option-input-theme"]').setValue("ninjago");
    await w.get('[data-testid="builder-option-suggestion-theme"]').trigger("click");
    expect(w.get('[data-testid="builder-option-feedback-theme"]').text()).toContain("Added “Ninjago”");

    await w.get('[data-testid="builder-option-input-theme"]').setValue("LEGO Ideas");
    await w.get('[data-testid="builder-option-add-theme"]').trigger("click");

    const preview = w.get('[data-testid="builder-preview"]');
    const select = preview.get('[data-testid="attribute-field-theme"]');
    expect(select.findAll("option").map((o) => o.text())).toEqual(["—", "Star Wars", "Ninjago", "LEGO Ideas"]);
    expect(preview.find('[data-testid="attribute-field-year"]').exists()).toBe(true);

    await w.get('[data-testid="builder-identifier"]').setValue("integer:year");
    await w.get('[data-testid="category-builder"]').trigger("submit");
    const saved = w.emitted("save")?.[0]?.[0] as CategoryDraft;
    expect(saved).toEqual({
      name: "Brick Sets",
      fields: [
        { id: "string:theme", options: ["Star Wars", "Ninjago", "LEGO Ideas"] },
        { id: "integer:year", options: [] },
      ],
      catalog_identifier: "integer:year",
    });
  });

  it("explains what's missing instead of saving", async () => {
    const w = mountBuilder();
    await w.get('[data-testid="builder-name"]').setValue("Shoes");
    await w.get('[data-testid="category-builder"]').trigger("submit");
    expect(w.get('[data-testid="builder-error"]').text()).toContain("Pick at least one field");
    expect(w.emitted("save")).toBeUndefined();
  });

  it("offers the existing category when the name is taken", () => {
    const w = mountBuilder({
      error: "You can already use a category called “Lego Sets”.",
      existing: { slug: "lego-sets", name: "Lego Sets" },
    });
    expect(w.get('[data-testid="builder-existing-link"]').attributes("href")).toBe("/categories/lego-sets");
  });

  it("starts from the category being edited", async () => {
    const initial: Category = {
      id: "c1", slug: "sneakers-abc123", name: "Sneakers", status: "private", mine: true, catalog_identifier: "theme",
      attribute_schema: { type: "object", properties: { theme: { type: "string", title: "Theme", enum: ["Nike"] } } },
    };
    const w = mountBuilder({ initial });
    expect((w.get('[data-testid="builder-name"]').element as HTMLInputElement).value).toBe("Sneakers");
    expect(w.get('[data-testid="builder-selected-theme"]').text()).toContain("Nike");
    await w.get('[data-testid="category-builder"]').trigger("submit");
    expect(w.emitted("save")?.[0]?.[0]).toMatchObject({
      fields: [{ id: "string:theme", options: ["Nike"] }],
      catalog_identifier: "string:theme",
    });
  });
});

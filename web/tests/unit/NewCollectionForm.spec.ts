import { describe, it, expect, vi, beforeEach } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import NewCollectionForm from "~/components/NewCollectionForm.vue";
import type { Category } from "~/types/api";

// Shared stub state per test — replaced in beforeEach.
let createMock = vi.fn();

beforeEach(() => {
  createMock = vi.fn(async (input: { categorySlug: string; name: string }) => ({
    id: "new-id",
    category_id: "cat-id",
    category_slug: input.categorySlug,
    category_name: "Category",
    name: input.name,
    item_count: 0,
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }));
  // @ts-expect-error test global
  globalThis.useMyCollectionsActions = () => ({
    create: createMock,
    rename: vi.fn(),
    remove: vi.fn(),
  });
});

const categories: Category[] = [
  { id: "c1", slug: "lego-sets", name: "Lego Sets", attribute_schema: {} },
  { id: "c2", slug: "funko-pops", name: "Funko Pops", attribute_schema: {} },
];

describe("NewCollectionForm", () => {
  it("renders a category option for each provided category", () => {
    const wrapper = mount(NewCollectionForm, { props: { categories } });
    const options = wrapper.findAll('[data-testid="new-collection-category"] option');
    expect(options).toHaveLength(categories.length);
    expect(options[0].text()).toBe("Lego Sets");
  });

  it("refuses to submit with an empty name", async () => {
    const wrapper = mount(NewCollectionForm, { props: { categories } });
    await wrapper.get('[data-testid="new-collection-submit"]').trigger("submit");
    await flushPromises();
    expect(createMock).not.toHaveBeenCalled();
    expect(wrapper.get('[data-testid="new-collection-error"]').text()).toContain("1–100");
  });

  it("submits the trimmed name and the selected category slug", async () => {
    const wrapper = mount(NewCollectionForm, { props: { categories } });
    await wrapper.get('[data-testid="new-collection-name"]').setValue("  Alice's Lego  ");
    await wrapper.get('[data-testid="new-collection-category"]').setValue("funko-pops");
    await wrapper.get('[data-testid="new-collection-submit"]').trigger("submit");
    await flushPromises();

    expect(createMock).toHaveBeenCalledTimes(1);
    expect(createMock).toHaveBeenCalledWith({ categorySlug: "funko-pops", name: "Alice's Lego" });

    const emitted = wrapper.emitted("created");
    expect(emitted).toBeTruthy();
    expect(emitted?.[0][0]).toMatchObject({ name: "Alice's Lego", category_slug: "funko-pops" });
  });

  it("shows an error message when the API call fails", async () => {
    createMock = vi.fn(async () => {
      throw new Error("boom");
    });
    // @ts-expect-error test global
    globalThis.useMyCollectionsActions = () => ({
      create: createMock,
      rename: vi.fn(),
      remove: vi.fn(),
    });

    const wrapper = mount(NewCollectionForm, { props: { categories } });
    await wrapper.get('[data-testid="new-collection-name"]').setValue("My Lego");
    await wrapper.get('[data-testid="new-collection-submit"]').trigger("submit");
    await flushPromises();

    expect(wrapper.get('[data-testid="new-collection-error"]').text()).toContain("Couldn’t create");
  });
});

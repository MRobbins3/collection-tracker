import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h, ref, type Ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import AddToCollection from "~/components/AddToCollection.vue";
import { installAuthStub, makeAuthStub } from "./authStub";
import type { CatalogEntry, CatalogOwnershipResponse, Collection, CollectionsResponse } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});
const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });
const SignInPromptStub = defineComponent({
  name: "SignInPrompt",
  props: { message: String },
  setup: (props) => () => h("div", { "data-testid": "signin-prompt" }, props.message),
});

const entry: CatalogEntry = {
  id: "entry-1",
  category_id: "cat-1",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
  name: "Millennium Falcon",
  identifier: "75192",
  attributes: { set_number: "75192" },
  source: "seed",
  status: "approved",
};

function collection(id: string, name: string, slug = "lego-sets"): Collection {
  return {
    id,
    name,
    category_id: "cat",
    category_slug: slug,
    category_name: "x",
    item_count: 0,
    created_at: "",
    updated_at: "",
  };
}

let collections: Ref<CollectionsResponse>;
let ownership: Ref<CatalogOwnershipResponse>;
let createCollection: ReturnType<typeof vi.fn>;
let addEntry: ReturnType<typeof vi.fn>;

beforeEach(() => {
  installAuthStub(
    makeAuthStub({
      user: { id: "u", email: "e", display_name: "U", handle: "u", avatar_color: "teal", created_at: "" },
    }),
  );
  collections = ref({ collections: [] });
  ownership = ref({ owned: [] });
  createCollection = vi.fn(async (input: { name: string }) => collection("new-col", input.name));
  addEntry = vi.fn(async () => ({}));
  // @ts-expect-error vitest globals standing in for Nuxt auto-imports
  globalThis.useMyCollections = () => ({ data: collections, pending: ref(false), refresh: vi.fn() });
  // @ts-expect-error vitest global standing in for a Nuxt auto-import
  globalThis.useCatalogOwnership = () => ({ data: ownership, refresh: vi.fn() });
  // @ts-expect-error vitest global standing in for a Nuxt auto-import
  globalThis.useMyCollectionsActions = () => ({ create: createCollection });
  // @ts-expect-error vitest global standing in for a Nuxt auto-import
  globalThis.useCatalogActions = () => ({ addEntry });
});

function mountPanel() {
  return mount(AddToCollection, {
    props: { entry },
    global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub, SignInPrompt: SignInPromptStub } },
  });
}

describe("AddToCollection", () => {
  it("asks anonymous visitors to sign in", () => {
    installAuthStub(makeAuthStub());
    const wrapper = mountPanel();
    expect(wrapper.get('[data-testid="signin-prompt"]').text()).toContain("Sign in to add Millennium Falcon");
  });

  it("offers your collections in this category, defaulting to the first", async () => {
    collections.value = {
      collections: [collection("c1", "Shelf"), collection("c2", "Bin"), collection("v1", "Records", "vinyl-records")],
    };
    const wrapper = mountPanel();
    await flushPromises();
    const radios = wrapper.findAll('input[type="radio"]');
    expect(radios).toHaveLength(3); // Shelf, Bin, and "A new collection"
    expect(wrapper.text()).not.toContain("Records");
    expect((radios[0]!.element as HTMLInputElement).checked).toBe(true);

    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(createCollection).not.toHaveBeenCalled();
    expect(addEntry).toHaveBeenCalledWith("c1", entry);
    expect(wrapper.get('[data-testid="add-to-collection-added"]').text()).toContain("Added to Shelf");
  });

  it("creates a new collection when you have none in this category", async () => {
    const wrapper = mountPanel();
    await flushPromises();
    expect((wrapper.get('[data-testid="add-to-collection-new"]').element as HTMLInputElement).checked).toBe(true);
    await wrapper.get('[data-testid="add-to-collection-name"]').setValue("My Lego");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(createCollection).toHaveBeenCalledWith({ categorySlug: "lego-sets", name: "My Lego" });
    expect(addEntry).toHaveBeenCalledWith("new-col", entry);
  });

  it("says where you already have it", async () => {
    ownership.value = {
      owned: [
        { catalog_entry_id: "entry-1", item_id: "i1", collection_id: "c1", collection_name: "Shelf" },
        { catalog_entry_id: "other", item_id: "i2", collection_id: "c2", collection_name: "Bin" },
      ],
    };
    const wrapper = mountPanel();
    const owned = wrapper.get('[data-testid="add-to-collection-owned"]');
    expect(owned.text()).toContain("In your collection:");
    expect(owned.text()).toContain("Shelf");
    expect(owned.text()).not.toContain("Bin");
  });

  it("shows an error when adding fails", async () => {
    addEntry.mockRejectedValue(new Error("boom"));
    vi.spyOn(console, "error").mockImplementation(() => {});
    const wrapper = mountPanel();
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("Couldn’t add it");
  });
});

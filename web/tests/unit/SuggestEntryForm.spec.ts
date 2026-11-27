import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import SuggestEntryForm from "~/components/SuggestEntryForm.vue";
import type { AttributeSchema, SubmitResult } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});

const schema: AttributeSchema = {
  type: "object",
  properties: { set_number: { type: "string", title: "Set number" } },
};

let submitEntry: ReturnType<typeof vi.fn>;

beforeEach(() => {
  submitEntry = vi.fn(
    async (): Promise<SubmitResult> => ({
      outcome: "submitted",
      item_linked: false,
      submission: {
        id: "req-1",
        kind: "catalog_entry",
        status: "pending",
        category_slug: "lego-sets",
        category_name: "Lego Sets",
        name: "Ultimate Tower",
        attributes: {},
        created_at: "",
      },
    }),
  );
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useSubmissionActions = () => ({ submitEntry, proposeCorrection: vi.fn() });
});

function mountForm(props: Record<string, unknown> = {}) {
  return mount(SuggestEntryForm, {
    props: { categorySlug: "lego-sets", schema, ...props },
    global: { stubs: { NuxtLink: NuxtLinkStub } },
  });
}

describe("SuggestEntryForm", () => {
  it("prefills from the item and sends the suggestion with the item to link", async () => {
    const wrapper = mountForm({ initialName: "Ultimate Tower", initialAttributes: { set_number: "10305" }, itemId: "item-1" });
    expect((wrapper.get('[data-testid="suggest-entry-name"]').element as HTMLInputElement).value).toBe("Ultimate Tower");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(submitEntry).toHaveBeenCalledWith({
      category_slug: "lego-sets",
      name: "Ultimate Tower",
      description: undefined,
      attributes: { set_number: "10305" },
      item_id: "item-1",
    });
    expect(wrapper.get('[data-testid="suggest-entry-result"]').text()).toContain("An admin will review");
    expect(wrapper.emitted("submitted")).toHaveLength(1);
  });

  it("offers Title Case for an all-lowercase name", async () => {
    const wrapper = mountForm({ initialName: "ultimate tower" });
    await wrapper.get('[data-testid="suggest-entry-title-case"]').trigger("click");
    expect((wrapper.get('[data-testid="suggest-entry-name"]').element as HTMLInputElement).value).toBe("Ultimate Tower");
  });

  it("tells you when it's already in the catalog", async () => {
    submitEntry.mockResolvedValue({
      outcome: "already_in_catalog",
      item_linked: true,
      entry: { id: "e1", name: "Millennium Falcon" },
    });
    const wrapper = mountForm({ initialName: "millennium falcon" });
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[data-testid="suggest-entry-result"]').text()).toContain(
      "“Millennium Falcon” is already in the catalog. Your item is now linked to it.",
    );
  });

  it("refuses a blank name without calling the API", async () => {
    const wrapper = mountForm();
    await wrapper.get("form").trigger("submit");
    expect(wrapper.get('[data-testid="suggest-entry-error"]').text()).toContain("Name must be");
    expect(submitEntry).not.toHaveBeenCalled();
  });

  it("shows the API's message when it refuses", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    submitEntry.mockRejectedValue({ data: { error: "You have 20 suggestions waiting for review." } });
    const wrapper = mountForm({ initialName: "Thing" });
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("20 suggestions waiting");
  });
});

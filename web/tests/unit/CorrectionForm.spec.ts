import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import CorrectionForm from "~/components/CorrectionForm.vue";
import type { AttributeSchema, CatalogEntry } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: String(props.to) }, slots.default?.());
  },
});
const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });

const schema: AttributeSchema = {
  type: "object",
  properties: {
    set_number: { type: "string", title: "Set number" },
    piece_count: { type: "integer", title: "Piece count" },
  },
};

const entry: CatalogEntry = {
  id: "e1",
  category_id: "c1",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
  name: "Millennium Falcon",
  identifier: "75192",
  attributes: { set_number: "75192", piece_count: 7541 },
  source: "seed",
  status: "approved",
};

let proposeCorrection: ReturnType<typeof vi.fn>;

beforeEach(() => {
  proposeCorrection = vi.fn(async () => ({ id: "req-1" }));
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useSubmissionActions = () => ({ submitEntry: vi.fn(), proposeCorrection });
});

async function openForm() {
  const wrapper = mount(CorrectionForm, {
    props: { entry, schema },
    global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub } },
  });
  await wrapper.get('[data-testid="correction-open"]').trigger("click");
  return wrapper;
}

describe("CorrectionForm", () => {
  it("sends only the fields you changed, plus your note", async () => {
    const wrapper = await openForm();
    await wrapper.get('[data-testid="attribute-field-piece_count"]').setValue("7500");
    await wrapper.get('[data-testid="correction-note"]').setValue("Counted twice");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(proposeCorrection).toHaveBeenCalledWith({
      catalog_entry_id: "e1",
      attributes: { set_number: "75192", piece_count: 7500 },
      note: "Counted twice",
    });
    expect(wrapper.get('[data-testid="correction-sent"]').text()).toContain("An admin will review your correction");
  });

  it("asks for a change before sending", async () => {
    const wrapper = await openForm();
    await wrapper.get("form").trigger("submit");
    expect(wrapper.get('[data-testid="correction-error"]').text()).toContain("Change at least one detail");
    expect(proposeCorrection).not.toHaveBeenCalled();
  });

  it("sends a renamed entry", async () => {
    const wrapper = await openForm();
    await wrapper.get('[data-testid="correction-name"]').setValue("Millennium Falcon UCS");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(proposeCorrection).toHaveBeenCalledWith({ catalog_entry_id: "e1", name: "Millennium Falcon UCS" });
  });
});

import { describe, it, expect } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ItemForm from "~/components/ItemForm.vue";
import type { AttributeSchema } from "~/types/api";

const schema: AttributeSchema = {
  type: "object",
  properties: {
    set_number: { type: "string", title: "Set number" },
    piece_count: { type: "integer", title: "Piece count" },
  },
};

describe("ItemForm", () => {
  it("rejects submission with an empty name", async () => {
    const wrapper = mount(ItemForm, { props: { schema } });
    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();
    expect(wrapper.emitted("submit")).toBeUndefined();
    expect(wrapper.get('[data-testid="item-form-error"]').text()).toContain("1–200");
  });

  it("submits trimmed name + quantity + condition + filled attributes only", async () => {
    const wrapper = mount(ItemForm, { props: { schema } });
    await wrapper.get('[data-testid="item-form-name"]').setValue("  Falcon  ");
    await wrapper.get('[data-testid="item-form-quantity"]').setValue(2);
    await wrapper.get('[data-testid="item-form-condition"]').setValue("New");
    await wrapper.get('[data-testid="attribute-field-set_number"]').setValue("75192");
    // Deliberately leave piece_count empty — it must not show up in the payload.

    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();

    const emitted = wrapper.emitted("submit");
    expect(emitted).toBeTruthy();
    expect(emitted?.[0][0]).toEqual({
      name: "Falcon",
      quantity: 2,
      condition: "New",
      attributes: { set_number: "75192" },
    });
  });

  it("pre-fills from the initial item when editing", () => {
    const wrapper = mount(ItemForm, {
      props: {
        schema,
        initial: {
          name: "Existing",
          quantity: 3,
          condition: "Mint",
          attributes: { set_number: "75192", piece_count: 7541 },
        },
      },
    });
    expect((wrapper.get('[data-testid="item-form-name"]').element as HTMLInputElement).value).toBe("Existing");
    expect((wrapper.get('[data-testid="item-form-quantity"]').element as HTMLInputElement).value).toBe("3");
    expect((wrapper.get('[data-testid="item-form-condition"]').element as HTMLInputElement).value).toBe("Mint");
    expect((wrapper.get('[data-testid="attribute-field-set_number"]').element as HTMLInputElement).value).toBe("75192");
  });

  it("emits cancel when the cancel button is clicked", async () => {
    const wrapper = mount(ItemForm, { props: { schema } });
    await wrapper.get('[data-testid="item-form-cancel"]').trigger("click");
    expect(wrapper.emitted("cancel")).toBeTruthy();
  });
});

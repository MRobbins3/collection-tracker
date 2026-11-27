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
    expect(emitted?.[0]?.[0]).toEqual({
      name: "Falcon",
      quantity: 2,
      condition: "New",
      attributes: { set_number: "75192" },
    });
  });

  it("collapses inner whitespace in the name, condition, and text attributes", async () => {
    const wrapper = mount(ItemForm, { props: { schema } });
    await wrapper.get('[data-testid="item-form-name"]').setValue("  Millennium \t Falcon   UCS ");
    await wrapper.get('[data-testid="item-form-condition"]').setValue(" New,   sealed ");
    await wrapper.get('[data-testid="attribute-field-set_number"]').setValue("  75192  ");
    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();
    expect(wrapper.emitted("submit")?.[0]?.[0]).toEqual({
      name: "Millennium Falcon UCS",
      quantity: 1,
      condition: "New, sealed",
      attributes: { set_number: "75192" },
    });
  });

  it("suggests Title Case for an all-lowercase name without forcing it", async () => {
    const wrapper = mount(ItemForm, { props: { schema } });
    await wrapper.get('[data-testid="item-form-name"]').setValue("millennium falcon");
    const tip = wrapper.get('[data-testid="item-form-title-case"]');
    expect(tip.text()).toContain("Millennium Falcon");

    // Keeping the lowercase name still submits it as typed.
    await wrapper.get('[data-testid="item-form-submit"]').trigger("submit");
    await flushPromises();
    expect(wrapper.emitted("submit")?.[0]?.[0]).toMatchObject({ name: "millennium falcon" });

    await tip.trigger("click");
    expect((wrapper.get('[data-testid="item-form-name"]').element as HTMLInputElement).value).toBe("Millennium Falcon");
    expect(wrapper.find('[data-testid="item-form-title-case"]').exists()).toBe(false);
  });

  it("offers no suggestion when the name already has deliberate capitals", async () => {
    const wrapper = mount(ItemForm, { props: { schema } });
    await wrapper.get('[data-testid="item-form-name"]').setValue("LEGO ucs falcon");
    expect(wrapper.find('[data-testid="item-form-title-case"]').exists()).toBe(false);
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

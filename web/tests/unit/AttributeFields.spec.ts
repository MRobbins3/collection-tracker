import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AttributeFields from "~/components/AttributeFields.vue";
import type { AttributeSchema } from "~/types/api";

const legoSchema: AttributeSchema = {
  type: "object",
  properties: {
    set_number: { type: "string", title: "Set number", description: "On the box" },
    piece_count: { type: "integer", title: "Piece count" },
    theme: { type: "string" }, // no title → humanize fallback
  },
};

describe("AttributeFields", () => {
  it("renders a labeled field per schema property, preferring title over raw key", () => {
    const wrapper = mount(AttributeFields, {
      props: { schema: legoSchema, modelValue: {} },
    });
    const text = wrapper.text();
    expect(text).toContain("Set number");
    expect(text).toContain("Piece count");
    expect(text).toContain("On the box");
    // Fallback humanization for the title-less property.
    expect(text).toContain("Theme");
    // Raw snake_case must not leak to the UI when a title exists.
    expect(text).not.toContain("set_number");
    expect(text).not.toContain("piece_count");
  });

  it("emits update:modelValue with the new value when a string field changes", async () => {
    const wrapper = mount(AttributeFields, {
      props: { schema: legoSchema, modelValue: {} },
    });
    const input = wrapper.get('[data-testid="attribute-field-set_number"]');
    await input.setValue("75192");
    const emitted = wrapper.emitted("update:modelValue");
    expect(emitted).toBeTruthy();
    expect(emitted?.[0][0]).toMatchObject({ set_number: "75192" });
  });

  it("coerces integer inputs to numbers", async () => {
    const wrapper = mount(AttributeFields, {
      props: { schema: legoSchema, modelValue: {} },
    });
    const input = wrapper.get('[data-testid="attribute-field-piece_count"]');
    await input.setValue("7541");
    const emitted = wrapper.emitted("update:modelValue");
    expect(emitted?.[0][0]).toMatchObject({ piece_count: 7541 });
  });

  it("renders nothing when the schema has no properties", () => {
    const wrapper = mount(AttributeFields, {
      props: { schema: { type: "object", properties: {} }, modelValue: {} },
    });
    expect(wrapper.find('[data-testid="attribute-fields"]').exists()).toBe(false);
  });
});

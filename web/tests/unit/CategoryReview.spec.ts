import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import CategoryReview from "~/components/CategoryReview.vue";
import type { ModerationRequest } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: String(props.to) }, slots.default?.());
  },
});

const request: ModerationRequest = {
  id: "r1",
  kind: "category",
  status: "pending",
  category_slug: "sneakers-abc123",
  category_name: "Sneakers",
  category: {
    id: "c1",
    slug: "sneakers-abc123",
    name: "Sneakers",
    description: "Shoes people collect",
    status: "pending",
    catalog_identifier: "theme",
    attribute_schema: {
      type: "object",
      properties: {
        theme: { type: "string", title: "Theme", enum: ["Nike", "Adidas"] },
        year: { type: "integer", title: "Year" },
      },
    },
  },
  note: "Popular hobby",
  submitter: { display_name: "Liam Johnson", handle: "liam" },
  created_at: "2026-11-27T10:00:00Z",
};

let admin: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  admin = {
    approve: vi.fn(async () => ({ ...request, status: "approved" })),
    reject: vi.fn(async () => ({ ...request, status: "rejected", reason: "Too narrow" })),
  };
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useModerationAdmin = () => admin;
});

function mountReview(r: ModerationRequest = request) {
  return mount(CategoryReview, { props: { request: r }, global: { stubs: { NuxtLink: NuxtLinkStub } } });
}

describe("CategoryReview", () => {
  it("shows what the category tracks and previews its item form, without raw keys", () => {
    const w = mountReview();
    const fields = w.get('[data-testid="review-category-fields"]').text();
    expect(fields).toContain("Theme");
    expect(fields).toContain("Pick list");
    expect(fields).toContain("Nike, Adidas");
    expect(fields).toContain("tells items apart");
    expect(w.text()).not.toContain("sneakers-abc123");
    expect(w.find('[data-testid="attribute-field-theme"]').element.tagName).toBe("SELECT");
  });

  it("approves as built", async () => {
    const w = mountReview();
    await w.get('[data-testid="review-approve"]').trigger("click");
    await flushPromises();
    expect(admin.approve).toHaveBeenCalledWith("r1");
    expect(w.emitted("decided")?.[0]?.[0]).toMatchObject({ status: "approved" });
  });

  it("needs a reason to reject", async () => {
    const w = mountReview();
    await w.get('[data-testid="review-reject"]').trigger("click");
    await w.get('[data-testid="review-reject-submit"]').trigger("submit");
    expect(w.get('[data-testid="review-error"]').text()).toContain("Say why");
    await w.get('[data-testid="review-reject-reason"]').setValue("  Too narrow ");
    await w.get('[data-testid="review-reject-submit"]').trigger("submit");
    await flushPromises();
    expect(admin.reject).toHaveBeenCalledWith("r1", "Too narrow");
  });

  it("shows a duplicate-name conflict with a reload", async () => {
    admin.approve = vi.fn(async () => {
      throw Object.assign(new Error("409"), {
        statusCode: 409,
        data: { error: "A category with this name is already on the public list." },
      });
    });
    const w = mountReview();
    await w.get('[data-testid="review-approve"]').trigger("click");
    await flushPromises();
    expect(w.get('[data-testid="review-error"]').text()).toContain("already on the public list");
    expect(w.get('[data-testid="review-error"]').find("button").exists()).toBe(true);
  });
});

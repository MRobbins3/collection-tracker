import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import ModerationReview from "~/components/ModerationReview.vue";
import type { AttributeSchema, CatalogEntry, ModerationRequest } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: String(props.to) }, slots.default?.());
  },
});

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
  name: "Ultimate Tower",
  identifier: "10305",
  attributes: { set_number: "10305", piece_count: 4514 },
  source: "user_submitted",
  status: "pending",
};

const submission: ModerationRequest = {
  id: "r1",
  kind: "catalog_entry",
  status: "pending",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
  entry,
  submitter: { display_name: "Liam Johnson", handle: "liam" },
  created_at: "2026-11-27T10:00:00Z",
};

const correction: ModerationRequest = {
  ...submission,
  id: "r2",
  kind: "catalog_correction",
  entry: { ...entry, status: "approved" },
  proposed: { name: "Lion Knights' Castle", attributes: { set_number: "10305", piece_count: 4500 } },
  note: "From the box",
};

let admin: Record<string, ReturnType<typeof vi.fn>>;
let search: ReturnType<typeof vi.fn>;

beforeEach(() => {
  admin = {
    approve: vi.fn(async () => ({ ...submission, status: "approved" })),
    reject: vi.fn(async () => ({ ...submission, status: "rejected", reason: "No" })),
    merge: vi.fn(async () => ({ ...submission, status: "merged" })),
  };
  search = vi.fn(async () => ({
    entries: [{ ...entry, id: "falcon", name: "Millennium Falcon", identifier: "75192", status: "approved" }],
    total: 1,
    has_more: false,
  }));
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useModerationAdmin = () => admin;
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useCatalogSearch = () => ({ search });
});

function mountReview(request: ModerationRequest) {
  return mount(ModerationReview, {
    props: { request, schema, identifierTitle: "Set number" },
    global: { stubs: { NuxtLink: NuxtLinkStub } },
  });
}

describe("ModerationReview", () => {
  it("shows a suggested entry with labelled details and approves it as submitted", async () => {
    const wrapper = mountReview(submission);
    const details = wrapper.get('[data-testid="review-submitted"]').text();
    expect(details).toContain("Piece count");
    expect(details).toContain("4514");
    expect(details).not.toContain("piece_count");
    await wrapper.get('[data-testid="review-approve"]').trigger("click");
    await flushPromises();
    expect(admin.approve).toHaveBeenCalledWith("r1");
    expect(wrapper.emitted("decided")?.[0]?.[0]).toMatchObject({ status: "approved" });
  });

  it("shows a correction as a before/after diff of only the changed details", () => {
    const wrapper = mountReview(correction);
    const rows = wrapper.findAll('[data-testid="review-diff-row"]').map((r) => r.text());
    expect(rows).toEqual(["NameUltimate TowerLion Knights' Castle", "Piece count45144500"]);
    expect(wrapper.text()).toContain("From the box");
    expect(wrapper.find('[data-testid="review-merge"]').exists()).toBe(false);
  });

  it("edit-then-approve sends the reviewer's version", async () => {
    const wrapper = mountReview(submission);
    await wrapper.get('[data-testid="review-edit"]').trigger("click");
    await wrapper.get('[data-testid="review-edit-name"]').setValue("Lion Knights' Castle");
    await wrapper.get('[data-testid="review-edit-submit"]').trigger("submit");
    await flushPromises();
    expect(admin.approve).toHaveBeenCalledWith("r1", {
      name: "Lion Knights' Castle",
      description: "",
      attributes: { set_number: "10305", piece_count: 4514 },
    });
  });

  it("rejecting needs a reason", async () => {
    const wrapper = mountReview(submission);
    await wrapper.get('[data-testid="review-reject"]').trigger("click");
    await wrapper.get('[data-testid="review-reject-submit"]').trigger("submit");
    expect(admin.reject).not.toHaveBeenCalled();
    expect(wrapper.get('[data-testid="review-error"]').text()).toContain("Say why");
    await wrapper.get('[data-testid="review-reject-reason"]').setValue("Please add the set number.");
    await wrapper.get('[data-testid="review-reject-submit"]').trigger("submit");
    await flushPromises();
    expect(admin.reject).toHaveBeenCalledWith("r1", "Please add the set number.");
  });

  it("merges a duplicate into the entry you pick", async () => {
    const wrapper = mountReview(submission);
    await wrapper.get('[data-testid="review-merge"]').trigger("click");
    await flushPromises();
    expect(search).toHaveBeenCalledWith("lego-sets", "Ultimate Tower");
    await wrapper.get('[data-testid="review-merge-option"] input').setValue(true);
    await wrapper.get('[data-testid="review-merge-submit"]').trigger("submit");
    await flushPromises();
    expect(admin.merge).toHaveBeenCalledWith("r1", "falcon", undefined);
  });

  it("explains a conflict when another admin decided first, and offers a reload", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    admin.approve!.mockRejectedValue({ statusCode: 409, data: { error: "Someone already decided this one." } });
    const wrapper = mountReview(submission);
    await wrapper.get('[data-testid="review-approve"]').trigger("click");
    await flushPromises();
    expect(wrapper.get('[data-testid="review-error"]').text()).toContain("Someone already decided");
    await wrapper.get('[data-testid="review-error"] button').trigger("click");
    expect(wrapper.emitted("reload")).toHaveLength(1);
  });

  it("shows the outcome and no actions once decided", () => {
    const wrapper = mountReview({
      ...submission,
      status: "rejected",
      reason: "Duplicate of an existing set.",
      decided_by: { display_name: "Olivia Martinez", handle: "olivia" },
      decided_at: "2026-11-27T11:00:00Z",
    });
    expect(wrapper.find('[data-testid="review-actions"]').exists()).toBe(false);
    const outcome = wrapper.get('[data-testid="review-outcome"]').text();
    expect(outcome).toContain("Not accepted by Olivia Martinez");
    expect(outcome).toContain("Duplicate of an existing set.");
  });
});

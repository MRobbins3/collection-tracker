import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import ModerationQueue from "~/components/ModerationQueue.vue";
import type { QueueItem, QueuePage } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: String(props.to) }, slots.default?.());
  },
});

function item(id: string, over: Partial<QueueItem> = {}): QueueItem {
  return {
    id,
    kind: "catalog_entry",
    category_slug: "lego-sets",
    category_name: "Lego Sets",
    name: `Set ${id}`,
    submitter: { display_name: "Liam Johnson", handle: "liam" },
    created_at: "2026-11-27T10:00:00Z",
    ...over,
  };
}

let queue: ReturnType<typeof vi.fn>;

beforeEach(() => {
  queue = vi.fn(async (): Promise<QueuePage> => ({ requests: [item("a"), item("b")], total: 3, has_more: true }));
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useModerationAdmin = () => ({ queue });
});

function mountQueue() {
  return mount(ModerationQueue, {
    props: { categories: [{ slug: "lego-sets", name: "Lego Sets" }, { slug: "plants", name: "Plants" }] },
    global: { stubs: { NuxtLink: NuxtLinkStub } },
  });
}

describe("ModerationQueue", () => {
  it("lists pending requests with kind, category, and submitter, linking to each", async () => {
    const wrapper = mountQueue();
    await flushPromises();
    expect(queue).toHaveBeenCalledWith({ kind: "", categorySlug: "", offset: 0 });
    const rows = wrapper.findAll('[data-testid="queue-item"]');
    expect(rows).toHaveLength(2);
    expect(rows[0]!.text()).toContain("New entry · Lego Sets");
    expect(rows[0]!.text()).toContain("From Liam Johnson");
    expect(rows[0]!.get("a").attributes("href")).toBe("/admin/moderation/a");
    expect(wrapper.get('[data-testid="queue-status"]').text()).toBe("3 waiting for review");
  });

  it("shows more from where it left off", async () => {
    const wrapper = mountQueue();
    await flushPromises();
    queue.mockResolvedValueOnce({ requests: [item("c", { kind: "catalog_correction" })], total: 3, has_more: false });
    await wrapper.get('[data-testid="queue-more"]').trigger("click");
    await flushPromises();
    expect(queue).toHaveBeenLastCalledWith({ kind: "", categorySlug: "", offset: 2 });
    expect(wrapper.findAll('[data-testid="queue-item"]')).toHaveLength(3);
    expect(wrapper.text()).toContain("Correction · Lego Sets");
    expect(wrapper.find('[data-testid="queue-more"]').exists()).toBe(false);
  });

  it("filters start over, and a slow answer for an old filter is dropped", async () => {
    const wrapper = mountQueue();
    await flushPromises();

    let resolveSlow!: (p: QueuePage) => void;
    queue.mockImplementationOnce(() => new Promise<QueuePage>((r) => (resolveSlow = r)));
    queue.mockResolvedValueOnce({ requests: [item("p", { category_name: "Plants" })], total: 1, has_more: false });

    await wrapper.get('[data-testid="queue-kind"]').setValue("catalog_entry");
    await wrapper.get('[data-testid="queue-category"]').setValue("plants");
    await flushPromises();
    expect(queue).toHaveBeenLastCalledWith({ kind: "catalog_entry", categorySlug: "plants", offset: 0 });

    resolveSlow({ requests: [item("stale")], total: 9, has_more: false });
    await flushPromises();
    const rows = wrapper.findAll('[data-testid="queue-item"]');
    expect(rows).toHaveLength(1);
    expect(rows[0]!.text()).toContain("Plants");
  });

  it("says when nothing is waiting", async () => {
    queue.mockResolvedValue({ requests: [], total: 0, has_more: false });
    const wrapper = mountQueue();
    await flushPromises();
    expect(wrapper.get('[data-testid="queue-empty"]').text()).toContain("Nothing waiting");
  });
});

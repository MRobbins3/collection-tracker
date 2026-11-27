import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { defineComponent, h, nextTick, ref, type Ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import ItemSearch from "~/components/ItemSearch.vue";
import type { ItemSearchResponse, ItemSearchResult } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: [String, Object], required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: typeof props.to === "string" ? props.to : "#" }, slots.default?.());
  },
});

const IconStub = defineComponent({
  name: "Icon",
  props: { name: { type: String, required: true } },
  setup() {
    return () => h("span");
  },
});

// useMyItemSearch stub: records the (debounced, normalized) term it was
// given and lets each test set what the "API" returned.
let term: Readonly<Ref<string>>;
let data: Ref<ItemSearchResponse>;
let pending: Ref<boolean>;
let error: Ref<Error | undefined>;
let refresh: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.useFakeTimers();
  data = ref({ items: [], has_more: false });
  pending = ref(false);
  error = ref(undefined);
  refresh = vi.fn();
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useMyItemSearch = (q: Readonly<Ref<string>>) => {
    term = q;
    return { data, pending, error, refresh };
  };
});

afterEach(() => {
  vi.useRealTimers();
});

function mountSearch(initial = "") {
  const wrapper = mount(ItemSearch, {
    props: {
      modelValue: initial,
      "onUpdate:modelValue": (v: string) => wrapper.setProps({ modelValue: v }),
    },
    global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub } },
  });
  return wrapper;
}

const falcon: ItemSearchResult = {
  id: "item-1",
  name: "Millennium Falcon",
  quantity: 2,
  collection_id: "col-1",
  collection_name: "Lego Shelf",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
};

describe("ItemSearch", () => {
  it("has a labelled search box and shows nothing until you type", () => {
    const wrapper = mountSearch();
    const input = wrapper.get('[data-testid="item-search-input"]');
    expect(input.attributes("type")).toBe("search");
    const id = input.attributes("id");
    expect(wrapper.get(`label[for="${id}"]`).text()).toBe("Search your items");
    expect(wrapper.find('[data-testid="item-search-results"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="item-search-status"]').text()).toBe("");
  });

  it("debounces and normalizes the query before searching", async () => {
    const wrapper = mountSearch();
    await wrapper.get('[data-testid="item-search-input"]').setValue("  millennium   falcon ");
    expect(wrapper.props("modelValue")).toBe("  millennium   falcon ");
    expect(term.value).toBe("");
    expect(wrapper.get('[data-testid="item-search-searching"]').text()).toContain("Searching");
    // Screen readers hear only the final count, not every debounced keystroke.
    expect(wrapper.get('[data-testid="item-search-status"]').text()).toBe("");

    vi.advanceTimersByTime(250);
    await nextTick();
    expect(term.value).toBe("millennium falcon");
  });

  it("starts from an existing query (e.g. back from a result) without waiting", () => {
    mountSearch("falcon");
    expect(term.value).toBe("falcon");
  });

  it("lists results that link to the item's collection, with human labels", async () => {
    data.value = {
      items: [
        falcon,
        {
          id: "item-2",
          name: "Abbey Road",
          quantity: 1,
          collection_id: "col-2",
          collection_name: "Records",
          category_slug: "vinyl-records",
          category_name: "Vinyl Records",
          matched_field: { label: "Artist", value: "The Beatles" },
        },
      ],
      has_more: false,
    };
    const wrapper = mountSearch("a");
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="item-search-result"]');
    expect(rows).toHaveLength(2);
    expect(rows[0]!.get("a").attributes("href")).toBe("/my/col-1");
    expect(rows[0]!.text()).toContain("Millennium Falcon");
    expect(rows[0]!.text()).toContain("Lego Shelf · Lego Sets");
    expect(rows[0]!.text()).toContain("×2");
    expect(rows[1]!.text()).toContain("Artist: The Beatles");
    expect(rows[1]!.text()).not.toContain("×1");
    expect(wrapper.get('[data-testid="item-search-status"]').text()).toBe("2 items found");
    expect(wrapper.text()).not.toContain("vinyl-records");
  });

  it("says when there are more matches than shown", async () => {
    data.value = { items: [falcon], has_more: true };
    const wrapper = mountSearch("falcon");
    await flushPromises();
    expect(wrapper.get('[data-testid="item-search-more"]').text()).toMatch(/more specific/i);
  });

  it("shows an empty state that repeats the query", async () => {
    const wrapper = mountSearch("  zebra   crossing ");
    await flushPromises();
    expect(wrapper.get('[data-testid="item-search-empty"]').text()).toContain("“zebra crossing”");
  });

  it("shows a searching state while the request is in flight", async () => {
    pending.value = true;
    const wrapper = mountSearch("falcon");
    await flushPromises();
    expect(wrapper.get('[data-testid="item-search-searching"]').text()).toContain("Searching");
    expect(wrapper.get('[data-testid="item-search-status"]').text()).toBe("");
    expect(wrapper.find('[data-testid="item-search-empty"]').exists()).toBe(false);
  });

  it("shows an error with a retry button", async () => {
    error.value = new Error("boom");
    const wrapper = mountSearch("falcon");
    await flushPromises();
    const box = wrapper.get('[data-testid="item-search-error"]');
    expect(box.text()).toContain("Couldn’t search");
    await box.get("button").trigger("click");
    expect(refresh).toHaveBeenCalledTimes(1);
  });
});

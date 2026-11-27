import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import PeopleSearch from "~/components/PeopleSearch.vue";
import type { PersonCard } from "~/types/api";
import { SEARCH_DEBOUNCE_MS } from "~/utils/search";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: String, required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: props.to }, slots.default?.());
  },
});
const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });

const person = (handle: string, display_name: string): PersonCard => ({ handle, display_name, avatar_color: "teal" });

let searchPeople: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.useFakeTimers();
  searchPeople = vi.fn(async (): Promise<PersonCard[]> => [person("noah", "Noah Williams")]);
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useSocialActions = () => ({ searchPeople });
});
afterEach(() => vi.useRealTimers());

function mountSearch() {
  return mount(PeopleSearch, { global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub } } });
}

async function type(wrapper: ReturnType<typeof mountSearch>, text: string) {
  await wrapper.get('[data-testid="people-search-input"]').setValue(text);
  vi.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
  await flushPromises();
}

describe("PeopleSearch", () => {
  it("searches once typing settles, without the @, and links results to profiles", async () => {
    const wrapper = mountSearch();
    await type(wrapper, "  @noah ");
    expect(searchPeople).toHaveBeenCalledTimes(1);
    expect(searchPeople).toHaveBeenCalledWith("noah");
    const results = wrapper.get('[data-testid="people-search-results"]');
    expect(results.text()).toContain("Noah Williams");
    expect(results.get("a").attributes("href")).toBe("/u/noah");
    expect(wrapper.get('[data-testid="people-search-status"]').text()).toBe("1 person found");
  });

  it("waits for at least two characters", async () => {
    const wrapper = mountSearch();
    await type(wrapper, "n");
    expect(searchPeople).not.toHaveBeenCalled();
    expect(wrapper.find('[data-testid="people-search-results"]').exists()).toBe(false);
  });

  it("says when no one matches", async () => {
    searchPeople.mockResolvedValueOnce([]);
    const wrapper = mountSearch();
    await type(wrapper, "zzz");
    expect(wrapper.get('[data-testid="people-search-empty"]').text()).toBe("No one matches “zzz”.");
  });

  it("drops a slow answer to an older search", async () => {
    let resolveOld!: (p: PersonCard[]) => void;
    searchPeople.mockImplementationOnce(() => new Promise<PersonCard[]>((r) => (resolveOld = r)));
    searchPeople.mockResolvedValueOnce([person("amelia", "Amelia Brown")]);
    const wrapper = mountSearch();
    await type(wrapper, "no");
    await type(wrapper, "amelia");
    resolveOld([person("noah", "Noah Williams")]);
    await flushPromises();
    const text = wrapper.get('[data-testid="people-search-results"]').text();
    expect(text).toContain("Amelia Brown");
    expect(text).not.toContain("Noah Williams");
  });
});

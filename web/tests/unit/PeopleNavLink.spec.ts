import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { defineComponent, h, reactive, ref } from "vue";
import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import PeopleNavLink from "~/components/PeopleNavLink.vue";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: String, required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: props.to }, slots.default?.());
  },
});
const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });

enableAutoUnmount(afterEach);

const count = ref(0);
const refreshCount = vi.fn(async () => {});
const route = reactive({ fullPath: "/" });

beforeEach(() => {
  count.value = 0;
  refreshCount.mockClear();
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useAuth = () => ({ isSignedIn: ref(true) });
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useRequestCount = () => ({ count, refreshCount });
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useRoute = () => route;
});

function mountLink() {
  return mount(PeopleNavLink, { global: { stubs: { NuxtLink: NuxtLinkStub, Icon: IconStub } } });
}

describe("PeopleNavLink", () => {
  it("links to People with no badge when nothing waits", async () => {
    const wrapper = mountLink();
    await flushPromises();
    expect(wrapper.get("a").attributes("href")).toBe("/people");
    expect(wrapper.text()).toContain("People");
    expect(wrapper.find('[data-testid="nav-people-badge"]').exists()).toBe(false);
    expect(refreshCount).toHaveBeenCalledTimes(1);
  });

  it("shows how many requests wait, with an accessible label", async () => {
    count.value = 3;
    const wrapper = mountLink();
    await flushPromises();
    expect(wrapper.get('[data-testid="nav-people-badge"]').text()).toBe("3, 3 requests waiting");
  });

  it("refreshes the count on every navigation", async () => {
    mountLink();
    await flushPromises();
    route.fullPath = "/my";
    await flushPromises();
    expect(refreshCount).toHaveBeenCalledTimes(2);
  });
});

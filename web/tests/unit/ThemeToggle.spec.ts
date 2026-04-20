import { describe, it, expect, beforeEach, vi } from "vitest";
import { ref, computed, defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import ThemeToggle from "~/components/ThemeToggle.vue";

// @nuxt/icon's <Icon> isn't available in Vitest's isolated environment.
// Stub it to a span that exposes the icon name via data-icon — lets us
// assert which icon was requested per theme state without loading Iconify.
const IconStub = defineComponent({
  name: "Icon",
  props: { name: { type: String, required: true } },
  setup(props) {
    return () => h("span", { "data-icon": props.name });
  },
});

// useTheme stub
let stub: ReturnType<typeof makeStub>;

function makeStub() {
  const preference = ref<"system" | "light" | "dark">("system");
  const systemDark = ref(false);
  const effective = computed<"light" | "dark">(() =>
    preference.value === "system" ? (systemDark.value ? "dark" : "light") : preference.value,
  );
  const cycle = vi.fn(() => {
    preference.value =
      preference.value === "system" ? "light" : preference.value === "light" ? "dark" : "system";
  });
  return { preference, systemDark, effective, cycle };
}

beforeEach(() => {
  stub = makeStub();
  // @ts-expect-error test global
  globalThis.useTheme = () => stub;
});

function mountWithStubs() {
  return mount(ThemeToggle, { global: { stubs: { Icon: IconStub } } });
}

describe("ThemeToggle", () => {
  it("starts in system preference with the monitor icon", () => {
    const wrapper = mountWithStubs();
    const button = wrapper.get('[data-testid="theme-toggle"]');
    expect(button.attributes("title")).toBe("System theme");
    expect(wrapper.find("[data-icon]").attributes("data-icon")).toBe("lucide:monitor");
  });

  it("cycles system → light → dark → system with matching icons", async () => {
    const wrapper = mountWithStubs();
    const button = wrapper.get('[data-testid="theme-toggle"]');

    await button.trigger("click");
    await wrapper.vm.$nextTick();
    expect(stub.preference.value).toBe("light");
    expect(button.attributes("title")).toBe("Light mode");
    expect(wrapper.find("[data-icon]").attributes("data-icon")).toBe("lucide:sun");

    await button.trigger("click");
    await wrapper.vm.$nextTick();
    expect(stub.preference.value).toBe("dark");
    expect(button.attributes("title")).toBe("Dark mode");
    expect(wrapper.find("[data-icon]").attributes("data-icon")).toBe("lucide:moon");

    await button.trigger("click");
    await wrapper.vm.$nextTick();
    expect(stub.preference.value).toBe("system");
    expect(wrapper.find("[data-icon]").attributes("data-icon")).toBe("lucide:monitor");
  });

  it("emits a distinct icon name per preference state", () => {
    const wrapper = mountWithStubs();
    stub.preference.value = "system";
    const systemName = wrapper.find("[data-icon]").attributes("data-icon");
    stub.preference.value = "light";
    // vue reactivity needs a tick
    return wrapper.vm.$nextTick().then(() => {
      const lightName = wrapper.find("[data-icon]").attributes("data-icon");
      stub.preference.value = "dark";
      return wrapper.vm.$nextTick().then(() => {
        const darkName = wrapper.find("[data-icon]").attributes("data-icon");
        expect(new Set([systemName, lightName, darkName]).size).toBe(3);
      });
    });
  });
});

import { describe, it, expect, beforeEach, vi } from "vitest";
import { ref, computed } from "vue";
import { mount } from "@vue/test-utils";
import ThemeToggle from "~/components/ThemeToggle.vue";

// useTheme stub — drives the component without loading the real composable.
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

describe("ThemeToggle", () => {
  it("starts with the system icon (default preference)", () => {
    const wrapper = mount(ThemeToggle);
    const button = wrapper.get('[data-testid="theme-toggle"]');
    expect(button.attributes("aria-label")).toContain("System theme");
    expect(button.attributes("title")).toBe("System theme");
  });

  it("cycles through system → light → dark → system", async () => {
    const wrapper = mount(ThemeToggle);
    const button = wrapper.get('[data-testid="theme-toggle"]');

    await button.trigger("click");
    expect(stub.cycle).toHaveBeenCalledTimes(1);
    expect(stub.preference.value).toBe("light");
    await wrapper.vm.$nextTick();
    expect(button.attributes("title")).toBe("Light mode");

    await button.trigger("click");
    expect(stub.preference.value).toBe("dark");
    await wrapper.vm.$nextTick();
    expect(button.attributes("title")).toBe("Dark mode");

    await button.trigger("click");
    expect(stub.preference.value).toBe("system");
    await wrapper.vm.$nextTick();
    expect(button.attributes("title")).toBe("System theme");
  });

  it("renders a distinct SVG per preference state", async () => {
    const wrapper = mount(ThemeToggle);
    const systemSvg = wrapper.find("svg").html();

    stub.preference.value = "light";
    await wrapper.vm.$nextTick();
    const lightSvg = wrapper.find("svg").html();

    stub.preference.value = "dark";
    await wrapper.vm.$nextTick();
    const darkSvg = wrapper.find("svg").html();

    expect(systemSvg).not.toBe(lightSvg);
    expect(lightSvg).not.toBe(darkSvg);
    expect(systemSvg).not.toBe(darkSvg);
  });
});

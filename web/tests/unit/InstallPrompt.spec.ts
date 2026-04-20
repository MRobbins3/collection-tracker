import { describe, it, expect, beforeEach, vi } from "vitest";
import { ref, computed } from "vue";
import { mount } from "@vue/test-utils";
import InstallPrompt from "~/components/InstallPrompt.vue";

// useInstall stub — lets us drive the surface computed directly.
let stub: ReturnType<typeof makeStub>;

function makeStub(initial: "android" | "ios" | "hidden" = "android") {
  const surfaceRef = ref<"android" | "ios" | "hidden">(initial);
  const promptInstall = vi.fn(async () => "accepted" as const);
  const dismiss = vi.fn(() => {
    surfaceRef.value = "hidden";
  });
  return {
    surfaceRef,
    surface: computed(() => surfaceRef.value),
    promptInstall,
    dismiss,
  };
}

beforeEach(() => {
  stub = makeStub("android");
  // @ts-expect-error test global
  globalThis.useInstall = () => ({
    surface: stub.surface,
    promptInstall: stub.promptInstall,
    dismiss: stub.dismiss,
    hydrate: vi.fn(),
  });
});

describe("InstallPrompt", () => {
  it("renders the Android install button when surface === 'android'", () => {
    const wrapper = mount(InstallPrompt);
    expect(wrapper.find('[data-testid="install-prompt-android"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="install-prompt-install"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="install-prompt-ios"]').exists()).toBe(false);
  });

  it("renders iOS Share instructions when surface === 'ios'", () => {
    stub.surfaceRef.value = "ios";
    const wrapper = mount(InstallPrompt);
    expect(wrapper.find('[data-testid="install-prompt-ios"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="install-prompt-install"]').exists()).toBe(false);
  });

  it("hides entirely when surface === 'hidden'", () => {
    stub.surfaceRef.value = "hidden";
    const wrapper = mount(InstallPrompt);
    expect(wrapper.find('[data-testid="install-prompt"]').exists()).toBe(false);
  });

  it("calls promptInstall when the install button is clicked", async () => {
    const wrapper = mount(InstallPrompt);
    await wrapper.get('[data-testid="install-prompt-install"]').trigger("click");
    expect(stub.promptInstall).toHaveBeenCalledOnce();
  });

  it("calls dismiss when the Dismiss button is clicked", async () => {
    const wrapper = mount(InstallPrompt);
    await wrapper.get('[data-testid="install-prompt-dismiss"]').trigger("click");
    expect(stub.dismiss).toHaveBeenCalledOnce();
  });
});

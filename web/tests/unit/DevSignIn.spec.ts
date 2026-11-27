import { describe, it, expect, beforeEach } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import DevSignIn from "~/components/DevSignIn.vue";
import { installAuthStub, makeAuthStub, type AuthStub } from "./authStub";

let stub: AuthStub;

beforeEach(() => {
  stub = makeAuthStub({ providers: { google: false, dev: true } });
  installAuthStub(stub);
});

async function openPicker(variant: "compact" | "full" = "full") {
  const wrapper = mount(DevSignIn, { props: { variant }, attachTo: document.body });
  await wrapper.get('[data-testid="dev-signin-button"]').trigger("click");
  await flushPromises();
  return wrapper;
}

describe("DevSignIn", () => {
  it("starts collapsed and does not fetch personas until opened", () => {
    const wrapper = mount(DevSignIn);
    const button = wrapper.get('[data-testid="dev-signin-button"]');
    expect(button.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find('[data-testid="dev-persona-picker"]').exists()).toBe(false);
    expect(stub.listPersonas).not.toHaveBeenCalled();
  });

  it("lists personas by display name, labeled as local-only test accounts", async () => {
    const wrapper = await openPicker();
    expect(wrapper.get('[data-testid="dev-signin-button"]').attributes("aria-expanded")).toBe("true");
    const picker = wrapper.get('[data-testid="dev-persona-picker"]');
    expect(picker.text()).toContain("Liam Johnson");
    expect(picker.text()).toContain("2 collections · 6 items");
    expect(picker.text()).toContain("Emma Garcia");
    expect(picker.text()).toContain("No collections yet");
    expect(picker.text()).toMatch(/local testing only/i);
    // No raw handles in the UI.
    expect(picker.text()).not.toMatch(/\bliam\b/);
    wrapper.unmount();
  });

  it("signs in as the chosen persona and closes", async () => {
    const wrapper = await openPicker();
    await wrapper.get('[data-testid="dev-persona-emma"]').trigger("click");
    await flushPromises();
    expect(stub.devLogin).toHaveBeenCalledWith("emma");
    expect(wrapper.find('[data-testid="dev-persona-picker"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it("shows an error when sign-in fails and stays open", async () => {
    stub.devLogin.mockRejectedValueOnce(new Error("boom"));
    const wrapper = await openPicker();
    await wrapper.get('[data-testid="dev-persona-liam"]').trigger("click");
    await flushPromises();
    expect(wrapper.get('[data-testid="dev-persona-error"]').text()).toMatch(/couldn.t sign in/i);
    expect(wrapper.find('[data-testid="dev-persona-picker"]').exists()).toBe(true);
    wrapper.unmount();
  });

  it("shows an error when personas fail to load", async () => {
    stub.listPersonas.mockRejectedValueOnce(new Error("boom"));
    const wrapper = await openPicker();
    expect(wrapper.get('[data-testid="dev-persona-error"]').text()).toMatch(/couldn.t load/i);
    wrapper.unmount();
  });

  it("explains an empty persona list", async () => {
    stub.listPersonas.mockResolvedValueOnce([]);
    const wrapper = await openPicker();
    expect(wrapper.get('[data-testid="dev-persona-empty"]').text()).toMatch(/no test users/i);
    wrapper.unmount();
  });

  it("closes on Escape and returns focus to the button", async () => {
    const wrapper = await openPicker("compact");
    await wrapper.get('[data-testid="dev-persona-picker"]').trigger("keydown", { key: "Escape" });
    expect(wrapper.find('[data-testid="dev-persona-picker"]').exists()).toBe(false);
    expect(document.activeElement).toBe(wrapper.get('[data-testid="dev-signin-button"]').element);
    wrapper.unmount();
  });
});

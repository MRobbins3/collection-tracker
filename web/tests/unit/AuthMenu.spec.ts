import { describe, it, expect, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount as baseMount } from "@vue/test-utils";
import AuthMenu from "~/components/AuthMenu.vue";
import { installAuthStub, makeAuthStub, type AuthStub } from "./authStub";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: String, required: true } },
  setup(props, { slots, attrs }) {
    return () => h("a", { ...attrs, href: props.to }, slots.default?.());
  },
});

function mount(component: typeof AuthMenu) {
  return baseMount(component, { global: { stubs: { NuxtLink: NuxtLinkStub } }, attachTo: document.body });
}

let stub: AuthStub;

beforeEach(() => {
  stub = makeAuthStub();
  installAuthStub(stub);
});

const alice = {
  id: "u1",
  email: "alice@example.com",
  display_name: "Alice Smith",
  handle: "alice",
  avatar_color: "rose",
  created_at: "2026-04-19T00:00:00Z",
};

describe("AuthMenu", () => {
  it("renders a Sign in with Google link when anonymous and providers are not known yet", () => {
    // SSR and the first client render: providers load after app:mounted, so
    // the markup must match the pre-dev-auth default exactly.
    const wrapper = mount(AuthMenu);
    const link = wrapper.get('[data-testid="signin-link"]');
    expect(link.attributes("href")).toBe("http://api.test/auth/google/start");
    expect(link.text()).toContain("Sign in with Google");
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(false);
  });

  it("shows only Google when only Google is configured", () => {
    stub.providers.value = { google: true, dev: false };
    const wrapper = mount(AuthMenu);
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(false);
  });

  it("shows only the test-user sign-in when dev auth is on and Google is not configured", () => {
    stub.providers.value = { google: false, dev: true };
    const wrapper = mount(AuthMenu);
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(false);
    expect(wrapper.get('[data-testid="dev-signin-button"]').text()).toContain("Test sign-in");
  });

  it("shows both when both are available", () => {
    stub.providers.value = { google: true, dev: true };
    const wrapper = mount(AuthMenu);
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(true);
  });

  it("keeps the Google link when neither provider is available", () => {
    stub.providers.value = { google: false, dev: false };
    const wrapper = mount(AuthMenu);
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(false);
  });

  it("signs in as a test user from the header picker", async () => {
    stub.providers.value = { google: false, dev: true };
    const wrapper = mount(AuthMenu);
    await wrapper.get('[data-testid="dev-signin-button"]').trigger("click");
    await flushPromises();
    await wrapper.get('[data-testid="dev-persona-liam"]').trigger("click");
    await flushPromises();
    expect(stub.devLogin).toHaveBeenCalledWith("liam");
    expect(wrapper.get('[data-testid="auth-menu-signed-in"]').text()).toContain("Liam Johnson");
  });

  it("renders an avatar menu button named for the user when signed in", () => {
    stub.user.value = alice;
    const wrapper = mount(AuthMenu);
    expect(wrapper.get('[data-testid="auth-menu-signed-in"]').text()).toContain("Alice Smith");
    const button = wrapper.get('[data-testid="account-menu-button"]');
    expect(button.text()).toContain("AS");
    expect(button.attributes("aria-expanded")).toBe("false");
    expect(button.attributes("aria-haspopup")).toBe("true");
    expect(wrapper.find('[data-testid="account-menu"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(false);
  });

  it("opens a menu linking to your profile and settings", async () => {
    stub.user.value = alice;
    const wrapper = mount(AuthMenu);
    const button = wrapper.get('[data-testid="account-menu-button"]');
    await button.trigger("click");
    expect(button.attributes("aria-expanded")).toBe("true");
    const menu = wrapper.get('[data-testid="account-menu"]');
    expect(button.attributes("aria-controls")).toBe(menu.attributes("id"));
    expect(menu.get('[data-testid="account-menu-profile"]').attributes("href")).toBe("/u/alice");
    expect(menu.get('[data-testid="account-menu-settings"]').attributes("href")).toBe("/settings");
    expect(menu.text()).toContain("@alice");
  });

  it("closes the menu on Escape", async () => {
    stub.user.value = alice;
    const wrapper = mount(AuthMenu);
    await wrapper.get('[data-testid="account-menu-button"]').trigger("click");
    await wrapper.get('[data-testid="account-menu"]').trigger("keydown", { key: "Escape" });
    expect(wrapper.find('[data-testid="account-menu"]').exists()).toBe(false);
  });

  it("calls logout when the Sign out button is clicked", async () => {
    stub.user.value = alice;
    const wrapper = mount(AuthMenu);
    await wrapper.get('[data-testid="account-menu-button"]').trigger("click");
    await wrapper.get('[data-testid="logout-button"]').trigger("click");
    expect(stub.logout).toHaveBeenCalledOnce();
    expect(stub.user.value).toBeNull();
    // After reactivity flush, the component should flip to the signed-out view.
    await wrapper.vm.$nextTick();
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(true);
  });
});

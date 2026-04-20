import { describe, it, expect, vi, beforeEach } from "vitest";
import { mount } from "@vue/test-utils";
import { ref, computed } from "vue";
import AuthMenu from "~/components/AuthMenu.vue";

// Hand-rolled auth stub. Easier and more honest than importing Nuxt runtime.
let stub = makeStub({ user: null });

function makeStub(init: { user: { id: string; email: string; display_name: string; created_at: string } | null }) {
  const user = ref(init.user);
  return {
    user,
    isSignedIn: computed(() => user.value !== null),
    loginURL: () => "http://api.test/auth/google/start",
    logout: vi.fn(async () => {
      user.value = null;
    }),
  };
}

// Inject our stub as a global auto-import named useAuth on a fresh globalThis.
beforeEach(() => {
  stub = makeStub({ user: null });
  // @ts-expect-error vitest global
  globalThis.useAuth = () => stub;
});

describe("AuthMenu", () => {
  it("renders a Sign in with Google link when anonymous", () => {
    const wrapper = mount(AuthMenu);
    const link = wrapper.get('[data-testid="signin-link"]');
    expect(link.attributes("href")).toBe("http://api.test/auth/google/start");
    expect(link.text()).toContain("Sign in with Google");
  });

  it("renders user name + logout button when signed in", () => {
    stub.user.value = {
      id: "u1",
      email: "alice@example.com",
      display_name: "Alice",
      created_at: "2026-04-19T00:00:00Z",
    };
    const wrapper = mount(AuthMenu);
    expect(wrapper.get('[data-testid="auth-menu-signed-in"]').text()).toContain("Alice");
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(false);
  });

  it("calls logout when the Sign out button is clicked", async () => {
    stub.user.value = {
      id: "u1",
      email: "a@example.com",
      display_name: "Alice",
      created_at: "2026-04-19T00:00:00Z",
    };
    const wrapper = mount(AuthMenu);
    await wrapper.get('[data-testid="logout-button"]').trigger("click");
    expect(stub.logout).toHaveBeenCalledOnce();
    expect(stub.user.value).toBeNull();
    // After reactivity flush, the component should flip to the signed-out view.
    await wrapper.vm.$nextTick();
    expect(wrapper.find('[data-testid="signin-link"]').exists()).toBe(true);
  });
});

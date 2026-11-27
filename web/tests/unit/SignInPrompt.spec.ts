import { describe, it, expect, beforeEach } from "vitest";
import { mount } from "@vue/test-utils";
import SignInPrompt from "~/components/SignInPrompt.vue";
import { installAuthStub, makeAuthStub, type AuthStub } from "./authStub";

let stub: AuthStub;

beforeEach(() => {
  stub = makeAuthStub();
  installAuthStub(stub);
});

describe("SignInPrompt", () => {
  it("shows the message and a Google link before providers are known", () => {
    const wrapper = mount(SignInPrompt, { props: { message: "Sign in to add items." } });
    expect(wrapper.text()).toContain("Sign in to add items.");
    expect(wrapper.get('[data-testid="signin-prompt-google"]').attributes("href")).toBe(
      "http://api.test/auth/google/start",
    );
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(false);
  });

  it("offers 'Sign in as a test user' instead of Google when only dev auth is on", () => {
    stub.providers.value = { google: false, dev: true };
    const wrapper = mount(SignInPrompt);
    expect(wrapper.find('[data-testid="signin-prompt-google"]').exists()).toBe(false);
    expect(wrapper.get('[data-testid="dev-signin-button"]').text()).toBe("Sign in as a test user");
  });

  it("offers both when both are available", () => {
    stub.providers.value = { google: true, dev: true };
    const wrapper = mount(SignInPrompt);
    expect(wrapper.find('[data-testid="signin-prompt-google"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(true);
  });

  it("is unchanged when only Google is configured", () => {
    stub.providers.value = { google: true, dev: false };
    const wrapper = mount(SignInPrompt);
    expect(wrapper.find('[data-testid="signin-prompt-google"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="dev-signin-button"]').exists()).toBe(false);
  });
});

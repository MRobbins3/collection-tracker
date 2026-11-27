import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ProfileForm from "~/components/ProfileForm.vue";
import type { HandleAvailability, Profile } from "~/types/api";

const base: Profile = {
  handle: "emma",
  display_name: "Emma Garcia",
  bio: "Just getting started.",
  avatar_color: "teal",
  collection_visibility: "private",
  accept_friend_requests: true,
  accept_follow_requests: true,
};

let update: ReturnType<typeof vi.fn>;
let checkHandle: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.useFakeTimers();
  update = vi.fn(async (patch: Partial<Profile>) => ({ ...base, ...patch }));
  checkHandle = vi.fn(
    async (h: string): Promise<HandleAvailability> =>
      h === "liam"
        ? { handle: h, available: false, message: "That handle is taken. Try another." }
        : { handle: h, available: true, message: "" },
  );
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useProfileActions = () => ({ update, checkHandle, deleteAccount: vi.fn() });
});

afterEach(() => {
  vi.useRealTimers();
});

function mountForm(profile: Profile = base) {
  return mount(ProfileForm, { props: { profile }, attachTo: document.body });
}

async function typeHandle(wrapper: ReturnType<typeof mountForm>, value: string) {
  await wrapper.get('[data-testid="profile-handle"]').setValue(value);
  await vi.advanceTimersByTimeAsync(400);
  await flushPromises();
}

describe("ProfileForm", () => {
  it("starts from the saved profile, with labelled fields", () => {
    const wrapper = mountForm();
    const name = wrapper.get('[data-testid="profile-display-name"]');
    expect((name.element as HTMLInputElement).value).toBe("Emma Garcia");
    expect(wrapper.get(`label[for="${name.attributes("id")}"]`).text()).toContain("Display name");
    expect((wrapper.get('[data-testid="profile-handle"]').element as HTMLInputElement).value).toBe("emma");
    expect((wrapper.get('[data-testid="visibility-private"]').element as HTMLInputElement).checked).toBe(true);
    expect(wrapper.get('[data-testid="profile-bio-count"]').text()).toBe("21/160");
  });

  it("offers Title Case for an all-lowercase display name, without forcing it", async () => {
    const wrapper = mountForm();
    await wrapper.get('[data-testid="profile-display-name"]').setValue("emma garcia");
    const suggestion = wrapper.get('[data-testid="display-name-suggestion"]');
    expect(suggestion.text()).toContain("Emma Garcia");
    await suggestion.trigger("click");
    expect((wrapper.get('[data-testid="profile-display-name"]').element as HTMLInputElement).value).toBe("Emma Garcia");
    expect(wrapper.find('[data-testid="display-name-suggestion"]').exists()).toBe(false);

    await wrapper.get('[data-testid="profile-display-name"]').setValue("EMMA G");
    expect(wrapper.find('[data-testid="display-name-suggestion"]').exists()).toBe(false);
  });

  it("checks a new handle after typing stops and says it's available", async () => {
    const wrapper = mountForm();
    await typeHandle(wrapper, "  @Emma_G ");
    expect(checkHandle).toHaveBeenCalledTimes(1);
    expect(checkHandle).toHaveBeenCalledWith("emma_g");
    expect(wrapper.get('[data-testid="handle-status"]').text()).toContain("@emma_g is available");
  });

  it("says when a handle is taken", async () => {
    const wrapper = mountForm();
    await typeHandle(wrapper, "liam");
    const status = wrapper.get('[data-testid="handle-status"]');
    expect(status.text()).toContain("That handle is taken");
    expect(wrapper.get('[data-testid="profile-handle"]').attributes("aria-invalid")).toBe("true");
  });

  it("asks you to slow down when the API rate-limits handle checks", async () => {
    checkHandle.mockRejectedValueOnce({
      statusCode: 429,
      data: { error: "You’re checking handles too quickly. Wait a moment and try again." },
    });
    const wrapper = mountForm();
    await typeHandle(wrapper, "emma_two");
    expect(wrapper.get('[data-testid="handle-status"]').text()).toContain("too quickly");
  });

  it("explains format problems without asking the server", async () => {
    const wrapper = mountForm();
    await typeHandle(wrapper, "has space");
    expect(checkHandle).not.toHaveBeenCalled();
    expect(wrapper.get('[data-testid="handle-status"]').text()).toContain("lowercase letters, numbers, and underscores");
    expect(wrapper.get('[data-testid="profile-save"]').attributes("disabled")).toBeDefined();
  });

  it("doesn't check your current handle", async () => {
    const wrapper = mountForm();
    await typeHandle(wrapper, "EMMA");
    expect(checkHandle).not.toHaveBeenCalled();
  });

  it("saves normalized values and the privacy choices", async () => {
    const wrapper = mountForm();
    await wrapper.get('[data-testid="profile-display-name"]').setValue("  Emma   G ");
    await wrapper.get('[data-testid="visibility-public"]').setValue(true);
    await wrapper.get('[data-testid="accept-follow-requests"]').setValue(false);
    await wrapper.get('[data-testid="profile-form"]').trigger("submit");
    await flushPromises();
    expect(update).toHaveBeenCalledWith({
      handle: "emma",
      display_name: "Emma G",
      bio: "Just getting started.",
      collection_visibility: "public",
      accept_friend_requests: true,
      accept_follow_requests: false,
    });
    expect(wrapper.get('[data-testid="profile-saved"]').text()).toContain("Saved");
    expect(wrapper.emitted("saved")?.[0]?.[0]).toMatchObject({ display_name: "Emma G" });
  });

  it("shows the server's per-field messages next to each field", async () => {
    update.mockRejectedValueOnce({
      data: {
        error: "Please fix the highlighted fields.",
        fields: { handle: "That handle is taken. Try another.", bio: "Bio must be 160 characters or fewer." },
      },
    });
    const wrapper = mountForm();
    await wrapper.get('[data-testid="profile-form"]').trigger("submit");
    await flushPromises();
    expect(wrapper.get('[data-testid="profile-handle-error"]').text()).toContain("taken");
    expect(wrapper.get('[data-testid="profile-bio-error"]').text()).toContain("160");
    expect(wrapper.find('[data-testid="profile-saved"]').exists()).toBe(false);
  });

  it("shows a general error when saving fails for another reason", async () => {
    update.mockRejectedValueOnce(new Error("network down"));
    const wrapper = mountForm();
    await wrapper.get('[data-testid="profile-form"]').trigger("submit");
    await flushPromises();
    expect(wrapper.get('[data-testid="profile-error"]').text()).toContain("couldn’t save");
  });

  it("explains each visibility level in plain words", () => {
    const wrapper = mountForm();
    const text = wrapper.get('[data-testid="privacy-fieldset"]').text();
    expect(text).toContain("Public");
    expect(text).toContain("Followers & friends");
    expect(text).toContain("Friends only");
    expect(text).toContain("Private");
    expect(text).not.toContain("followers_and_friends");
  });
});

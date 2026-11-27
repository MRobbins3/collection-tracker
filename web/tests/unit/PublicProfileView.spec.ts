import { describe, it, expect } from "vitest";
import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import PublicProfileView from "~/components/PublicProfileView.vue";
import type { PublicProfileResponse } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: String, required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: props.to }, slots.default?.());
  },
});

const liamCollections = [
  {
    id: "d0000000-0000-4000-8000-000000000101",
    category_id: "cat-1",
    category_slug: "lego-sets",
    category_name: "Lego Sets",
    name: "Lego Display Shelf",
    item_count: 4,
    created_at: "2026-11-27T00:00:00Z",
    updated_at: "2026-11-27T00:00:00Z",
  },
];

function response(over: Partial<PublicProfileResponse> = {}): PublicProfileResponse {
  return {
    profile: { handle: "liam", display_name: "Liam Johnson", bio: "Lego builder.", avatar_color: "sky" },
    is_self: false,
    collection_visibility: "public",
    collections_visible: true,
    collections: liamCollections,
    ...over,
  };
}

function mountView(data: PublicProfileResponse) {
  return mount(PublicProfileView, { props: { data }, global: { stubs: { NuxtLink: NuxtLinkStub } } });
}

describe("PublicProfileView", () => {
  it("shows the profile card with an initials avatar", () => {
    const wrapper = mountView(response());
    expect(wrapper.get("h1").text()).toBe("Liam Johnson");
    expect(wrapper.text()).toContain("@liam");
    expect(wrapper.text()).toContain("Lego builder.");
    expect(wrapper.get('[data-testid="user-avatar"]').text()).toBe("LJ");
  });

  it("lists visible collections read-only, linking to the read-only collection page", () => {
    const wrapper = mountView(response());
    const link = wrapper.get('[data-testid="profile-collections"] a');
    expect(link.attributes("href")).toBe("/u/liam/c/d0000000-0000-4000-8000-000000000101");
    expect(link.text()).toContain("Lego Display Shelf");
    expect(link.text()).toContain("Lego Sets · 4 items");
    expect(wrapper.find("button").exists()).toBe(false);
    expect(wrapper.find('[data-testid="profile-edit-link"]').exists()).toBe(false);
  });

  it("explains hidden collections in plain words", () => {
    const friends = mountView(
      response({ collection_visibility: "friends_only", collections_visible: false, collections: [] }),
    );
    expect(friends.get('[data-testid="profile-collections-hidden"]').text()).toContain(
      "Only Liam Johnson’s friends can see their collections.",
    );
    const followers = mountView(
      response({ collection_visibility: "followers_and_friends", collections_visible: false, collections: [] }),
    );
    expect(followers.get('[data-testid="profile-collections-hidden"]').text()).toContain(
      "Only Liam Johnson’s followers and friends can see their collections.",
    );
  });

  it("has an empty state", () => {
    const wrapper = mountView(response({ collections: [] }));
    expect(wrapper.get('[data-testid="profile-collections-empty"]').text()).toContain("No collections yet");
  });

  it("shows the owner an edit link and who can see their collections", () => {
    const wrapper = mountView(response({ is_self: true, collection_visibility: "private" }));
    expect(wrapper.get('[data-testid="profile-edit-link"]').attributes("href")).toBe("/settings");
    expect(wrapper.get('[data-testid="profile-visibility-note"]').text()).toContain("Only you");
  });
});

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
    relationship: null,
    ...over,
  };
}

const RelationshipActionsStub = defineComponent({
  name: "RelationshipActions",
  props: { handle: String, name: String, relationship: Object },
  emits: ["changed"],
  setup(props, { emit }) {
    return () =>
      h("button", { "data-testid": "relationship-stub", onClick: () => emit("changed") }, `${props.handle}:${props.name}`);
  },
});

function mountView(data: PublicProfileResponse) {
  return mount(PublicProfileView, {
    props: { data },
    global: { stubs: { NuxtLink: NuxtLinkStub, RelationshipActions: RelationshipActionsStub } },
  });
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

  it("shows friend and follow buttons to a signed-in visitor and passes on changes", async () => {
    const wrapper = mountView(
      response({
        relationship: {
          friend: "none",
          following: "none",
          follows_you: "none",
          accepts_friend_requests: true,
          accepts_follow_requests: true,
        },
      }),
    );
    const stub = wrapper.get('[data-testid="relationship-stub"]');
    expect(stub.text()).toBe("liam:Liam Johnson");
    await stub.trigger("click");
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  it("shows no relationship buttons when signed out or on your own profile", () => {
    expect(mountView(response()).find('[data-testid="relationship-stub"]').exists()).toBe(false);
    expect(mountView(response({ is_self: true })).find('[data-testid="relationship-stub"]').exists()).toBe(false);
  });

  it("offers Compare collections to a signed-in visitor who can see the collections", () => {
    const rel = {
      friend: "friends" as const,
      following: "none" as const,
      follows_you: "none" as const,
      accepts_friend_requests: true,
      accepts_follow_requests: true,
    };
    const link = mountView(response({ relationship: rel })).get('[data-testid="profile-compare-link"]');
    expect(link.attributes("href")).toBe("/u/liam/compare");
    expect(link.text()).toContain("Compare collections");

    const none = '[data-testid="profile-compare-link"]';
    expect(mountView(response()).find(none).exists(), "signed out").toBe(false);
    expect(mountView(response({ is_self: true })).find(none).exists(), "yourself").toBe(false);
    expect(
      mountView(response({ relationship: rel, collections_visible: false, collections: [] })).find(none).exists(),
      "hidden collections",
    ).toBe(false);
  });
});

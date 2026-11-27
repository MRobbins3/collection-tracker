import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import RelationshipActions from "~/components/RelationshipActions.vue";
import type { ViewerRelationship } from "~/types/api";

const IconStub = defineComponent({ name: "Icon", setup: () => () => h("span") });

function rel(over: Partial<ViewerRelationship> = {}): ViewerRelationship {
  return {
    friend: "none",
    following: "none",
    follows_you: "none",
    accepts_friend_requests: true,
    accepts_follow_requests: true,
    ...over,
  };
}

let actions: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  const ok = () => vi.fn(async () => {});
  actions = {
    sendFriendRequest: ok(),
    cancelFriendRequest: ok(),
    acceptFriendRequest: ok(),
    declineFriendRequest: ok(),
    unfriend: ok(),
    requestFollow: ok(),
    unfollow: ok(),
    approveFollower: ok(),
    denyFollower: ok(),
  };
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useSocialActions = () => actions;
});

function mountActions(relationship: ViewerRelationship) {
  return mount(RelationshipActions, {
    props: { handle: "noah", name: "Noah Williams", relationship },
    global: { stubs: { Icon: IconStub } },
  });
}

const tid = (id: string) => `[data-testid="${id}"]`;

describe("RelationshipActions", () => {
  it("offers Add friend and Request to follow to a stranger", async () => {
    const wrapper = mountActions(rel());
    await wrapper.get(tid("friend-add")).trigger("click");
    await flushPromises();
    expect(actions.sendFriendRequest).toHaveBeenCalledWith("noah");
    expect(wrapper.emitted("changed")).toHaveLength(1);

    await wrapper.get(tid("follow-request")).trigger("click");
    await flushPromises();
    expect(actions.requestFollow).toHaveBeenCalledWith("noah");
  });

  it("says so instead of offering a request someone has turned off", () => {
    const wrapper = mountActions(rel({ accepts_friend_requests: false, accepts_follow_requests: false }));
    expect(wrapper.find(tid("friend-add")).exists()).toBe(false);
    expect(wrapper.find(tid("follow-request")).exists()).toBe(false);
    expect(wrapper.get(tid("friend-closed")).text()).toBe("Not accepting friend requests");
    expect(wrapper.get(tid("follow-closed")).text()).toBe("Not accepting follow requests");
  });

  it("shows a sent request as pending, with a way to cancel it", async () => {
    const wrapper = mountActions(rel({ friend: "request_sent", following: "requested" }));
    expect(wrapper.get(tid("friend-status")).text()).toContain("Friend request sent");
    expect(wrapper.get(tid("follow-status")).text()).toContain("Follow requested");
    await wrapper.get(tid("friend-cancel")).trigger("click");
    await wrapper.get(tid("follow-cancel")).trigger("click");
    await flushPromises();
    expect(actions.cancelFriendRequest).toHaveBeenCalledWith("noah");
    expect(actions.unfollow).toHaveBeenCalledWith("noah");
  });

  it("lets you accept or decline a friend request they sent", async () => {
    const wrapper = mountActions(rel({ friend: "request_received" }));
    expect(wrapper.text()).toContain("Noah Williams wants to be friends");
    await wrapper.get(tid("friend-accept")).trigger("click");
    await flushPromises();
    expect(actions.acceptFriendRequest).toHaveBeenCalledWith("noah");
    await wrapper.get(tid("friend-decline")).trigger("click");
    await flushPromises();
    expect(actions.declineFriendRequest).toHaveBeenCalledWith("noah");
  });

  it("lets you approve or deny their follow request", async () => {
    const wrapper = mountActions(rel({ follows_you: "requested" }));
    expect(wrapper.get(tid("follow-request-banner")).text()).toContain("Noah Williams asked to follow you");
    await wrapper.get(tid("follower-approve")).trigger("click");
    await wrapper.get(tid("follower-deny")).trigger("click");
    await flushPromises();
    expect(actions.approveFollower).toHaveBeenCalledWith("noah");
    expect(actions.denyFollower).toHaveBeenCalledWith("noah");
  });

  it("confirms before unfriending or unfollowing", async () => {
    const wrapper = mountActions(rel({ friend: "friends", following: "following", follows_you: "following" }));
    expect(wrapper.get(tid("friend-status")).text()).toContain("Friends");
    expect(wrapper.get(tid("follow-status")).text()).toContain("Following");
    expect(wrapper.get(tid("follows-you")).text()).toBe("Follows you");

    await wrapper.get(tid("friend-remove")).trigger("click");
    expect(actions.unfriend).not.toHaveBeenCalled();
    expect(wrapper.get(tid("confirm-message")).text()).toBe("Unfriend Noah Williams?");
    await wrapper.get(tid("confirm-keep")).trigger("click");
    expect(wrapper.find(tid("confirm-message")).exists()).toBe(false);

    await wrapper.get(tid("friend-remove")).trigger("click");
    await wrapper.get(tid("confirm-yes")).trigger("click");
    await flushPromises();
    expect(actions.unfriend).toHaveBeenCalledWith("noah");

    await wrapper.get(tid("follow-remove")).trigger("click");
    expect(wrapper.get(tid("confirm-message")).text()).toBe("Unfollow Noah Williams?");
    await wrapper.get(tid("confirm-yes")).trigger("click");
    await flushPromises();
    expect(actions.unfollow).toHaveBeenCalledWith("noah");
  });

  it("shows the API's message when an action fails, and doesn't report a change", async () => {
    actions.sendFriendRequest!.mockRejectedValueOnce({
      statusCode: 403,
      data: { error: "@noah isn’t accepting friend requests." },
    });
    const wrapper = mountActions(rel());
    await wrapper.get(tid("friend-add")).trigger("click");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toBe("@noah isn’t accepting friend requests.");
    expect(wrapper.emitted("changed")).toBeUndefined();
  });

  it("disables every button while an action is in flight", async () => {
    let finish!: () => void;
    actions.sendFriendRequest!.mockImplementationOnce(() => new Promise<void>((r) => (finish = r)));
    const wrapper = mountActions(rel());
    await wrapper.get(tid("friend-add")).trigger("click");
    for (const b of wrapper.findAll("button")) expect(b.attributes("disabled")).toBeDefined();
    finish();
    await flushPromises();
    for (const b of wrapper.findAll("button")) expect(b.attributes("disabled")).toBeUndefined();
  });
});

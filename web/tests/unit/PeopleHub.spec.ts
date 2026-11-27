import { describe, it, expect, vi, beforeEach } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import PeopleHub from "~/components/PeopleHub.vue";
import type { PeopleTab, Person, RequestsResponse } from "~/types/api";

const NuxtLinkStub = defineComponent({
  name: "NuxtLink",
  props: { to: { type: String, required: true } },
  setup(props, { slots }) {
    return () => h("a", { href: props.to }, slots.default?.());
  },
});

const person = (handle: string, display_name: string): Person => ({
  handle,
  display_name,
  avatar_color: "teal",
  since: "2026-11-27T00:00:00Z",
});
const noah = person("noah", "Noah Williams");
const amelia = person("amelia", "Amelia Brown");
const emma = person("emma", "Emma Garcia");

let actions: Record<string, ReturnType<typeof vi.fn>>;
let requests: RequestsResponse;

beforeEach(() => {
  requests = { friend_incoming: [emma], friend_outgoing: [noah], follow_incoming: [amelia], follow_outgoing: [] };
  const ok = () => vi.fn(async () => {});
  actions = {
    listFriends: vi.fn(async () => [noah]),
    listFollowers: vi.fn(async () => [amelia]),
    listFollowing: vi.fn(async () => []),
    listRequests: vi.fn(async () => requests),
    unfriend: ok(),
    removeFollower: ok(),
    unfollow: ok(),
    acceptFriendRequest: ok(),
    declineFriendRequest: ok(),
    approveFollower: ok(),
    denyFollower: ok(),
    cancelFriendRequest: ok(),
  };
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useSocialActions = () => actions;
});

function mountHub(tab: PeopleTab) {
  return mount(PeopleHub, {
    props: { tab, "onUpdate:tab": vi.fn() },
    global: { stubs: { NuxtLink: NuxtLinkStub } },
    attachTo: document.body,
  });
}

const tid = (id: string) => `[data-testid="${id}"]`;

describe("PeopleHub", () => {
  it("lists friends, linking to each profile, and shows how many requests wait", async () => {
    const wrapper = mountHub("friends");
    await flushPromises();
    const rows = wrapper.findAll(tid("person-row"));
    expect(rows).toHaveLength(1);
    expect(rows[0]!.text()).toContain("Noah Williams");
    expect(rows[0]!.text()).toContain("@noah");
    expect(rows[0]!.get("a").attributes("href")).toBe("/u/noah");
    expect(wrapper.get(tid("people-tab-requests-count")).text()).toContain("2");
    expect(wrapper.get(tid("people-tab-friends")).attributes("aria-selected")).toBe("true");
  });

  it("asks before unfriending, then reloads the list", async () => {
    const wrapper = mountHub("friends");
    await flushPromises();
    await wrapper.get(tid("friend-remove")).trigger("click");
    expect(wrapper.get(tid("confirm-message")).text()).toBe("Unfriend Noah Williams?");
    expect(actions.unfriend).not.toHaveBeenCalled();
    actions.listFriends!.mockResolvedValueOnce([]);
    await wrapper.get(tid("confirm-yes")).trigger("click");
    await flushPromises();
    expect(actions.unfriend).toHaveBeenCalledWith("noah");
    expect(wrapper.get(tid("people-empty-friends")).text()).toContain("No friends yet");
  });

  it("removes a follower after confirming", async () => {
    const wrapper = mountHub("followers");
    await flushPromises();
    await wrapper.get(tid("follower-remove")).trigger("click");
    expect(wrapper.get(tid("confirm-message")).text()).toContain("Remove Amelia Brown from your followers?");
    await wrapper.get(tid("confirm-yes")).trigger("click");
    await flushPromises();
    expect(actions.removeFollower).toHaveBeenCalledWith("amelia");
  });

  it("shows an empty Following tab", async () => {
    const wrapper = mountHub("following");
    await flushPromises();
    expect(wrapper.get(tid("people-empty-following")).text()).toBe("You don’t follow anyone yet.");
  });

  it("answers incoming requests and cancels outgoing ones", async () => {
    const wrapper = mountHub("requests");
    await flushPromises();
    expect(wrapper.get(tid("requests-friend-incoming")).text()).toContain("Emma Garcia");
    expect(wrapper.get(tid("requests-follow-incoming")).text()).toContain("Amelia Brown");
    expect(wrapper.get(tid("requests-outgoing")).text()).toContain("Noah Williams");

    await wrapper.get(tid("friend-accept")).trigger("click");
    await flushPromises();
    expect(actions.acceptFriendRequest).toHaveBeenCalledWith("emma");
    await wrapper.get(tid("follower-deny")).trigger("click");
    await flushPromises();
    expect(actions.denyFollower).toHaveBeenCalledWith("amelia");
    await wrapper.get(tid("friend-cancel")).trigger("click");
    await flushPromises();
    expect(actions.cancelFriendRequest).toHaveBeenCalledWith("noah");
    expect(actions.listRequests).toHaveBeenCalledTimes(4);
  });

  it("says why an action failed", async () => {
    actions.approveFollower!.mockRejectedValueOnce({ statusCode: 404 });
    const wrapper = mountHub("requests");
    await flushPromises();
    await wrapper.get(tid("follower-approve")).trigger("click");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toBe("That person or request isn’t available any more.");
  });

  it("offers a retry when a list fails to load", async () => {
    actions.listFriends!.mockRejectedValueOnce(new Error("down"));
    const wrapper = mountHub("friends");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("We couldn’t load this list.");
    await wrapper.get('[role="alert"] button').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(tid("person-row"))).toHaveLength(1);
  });

  it("switches tabs by click and arrow key, reporting the new tab", async () => {
    const wrapper = mountHub("friends");
    await flushPromises();
    await wrapper.get(tid("people-tab-followers")).trigger("click");
    await wrapper.get(tid("people-tab-friends")).trigger("keydown", { key: "ArrowLeft" });
    expect(wrapper.emitted("update:tab")).toEqual([["followers"], ["requests"]]);
  });

  it("ignores a slow, outdated answer", async () => {
    let resolveOld!: (p: Person[]) => void;
    actions.listFriends!.mockImplementationOnce(() => new Promise<Person[]>((r) => (resolveOld = r)));
    const wrapper = mountHub("friends");
    await wrapper.setProps({ tab: "followers" });
    await wrapper.setProps({ tab: "friends" }); // a second, fast friends load
    await flushPromises();
    resolveOld([emma]);
    await flushPromises();
    const rows = wrapper.findAll(tid("person-row"));
    expect(rows.map((r) => r.text()).join()).toContain("Noah Williams");
    expect(rows.map((r) => r.text()).join()).not.toContain("Emma Garcia");
  });
});

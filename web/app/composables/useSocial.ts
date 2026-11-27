import type { Person, PersonCard, RequestsResponse } from "~/types/api";

/**
 * How many friend and follow requests wait for you (the header badge).
 * Starts at 0 on the server and the client alike (hydration parity); the
 * badge loads it in the browser once you're signed in.
 */
export function useRequestCount() {
  const count = useState<number>("social:requestCount", () => 0);
  const api = useApi();
  const { isSignedIn } = useAuth();

  async function refreshCount(): Promise<void> {
    if (!isSignedIn.value) {
      count.value = 0;
      return;
    }
    try {
      const { incoming } = await api.get<{ incoming: number }>("/me/requests/count");
      count.value = incoming;
    } catch {
      // Keep the last known count; the badge is a hint, not a source of truth.
    }
  }

  return { count, refreshCount };
}

/**
 * Friends and followers (ADR 0014): your lists, people search, and every
 * request action. Handles go in the URL; the API resolves them and applies
 * the privacy rule. Each action refreshes the header badge.
 */
export function useSocialActions() {
  const api = useApi();
  const { refreshCount } = useRequestCount();

  async function send(method: "POST" | "DELETE", path: string): Promise<void> {
    await $fetch(`${api.publicBaseURL}${path}`, { method, credentials: "include" });
    void refreshCount();
  }
  const u = (handle: string) => encodeURIComponent(handle);
  const people = async (path: string) => (await api.get<{ people: Person[] }>(path)).people;

  return {
    // Requests you send, from someone's profile.
    sendFriendRequest: (handle: string) => send("POST", `/users/${u(handle)}/friend-request`),
    cancelFriendRequest: (handle: string) => send("DELETE", `/users/${u(handle)}/friend-request`),
    requestFollow: (handle: string) => send("POST", `/users/${u(handle)}/follow`),
    /** Unfollow, or withdraw a pending follow request. */
    unfollow: (handle: string) => send("DELETE", `/users/${u(handle)}/follow`),
    // Answers and removals: only ever your own relationships.
    acceptFriendRequest: (handle: string) => send("POST", `/me/friend-requests/${u(handle)}/accept`),
    declineFriendRequest: (handle: string) => send("POST", `/me/friend-requests/${u(handle)}/decline`),
    unfriend: (handle: string) => send("DELETE", `/me/friends/${u(handle)}`),
    approveFollower: (handle: string) => send("POST", `/me/follow-requests/${u(handle)}/approve`),
    denyFollower: (handle: string) => send("POST", `/me/follow-requests/${u(handle)}/deny`),
    removeFollower: (handle: string) => send("DELETE", `/me/followers/${u(handle)}`),
    // Lists.
    listFriends: () => people("/me/friends"),
    listFollowers: () => people("/me/followers"),
    listFollowing: () => people("/me/following"),
    listRequests: () => api.get<RequestsResponse>("/me/requests"),
    /** Profiles you're allowed to see whose handle or name matches. */
    searchPeople: async (q: string) => (await api.get<{ people: PersonCard[] }>("/people", { q })).people,
  };
}

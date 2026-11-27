<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from "vue";
import PersonRow from "./PersonRow.vue";
import type { PeopleTab, Person, RequestsResponse } from "~/types/api";
import { socialErrorMessage } from "~/utils/social";

/**
 * Your friends, followers, who you follow, and pending requests, as tabs
 * (ADR 0014). The parent keeps the tab in the URL (v-model:tab). Lists load
 * when their tab opens and reload after every action.
 */
const props = defineProps<{ tab: PeopleTab }>();
const emit = defineEmits<{ (e: "update:tab", tab: PeopleTab): void }>();

const actions = useSocialActions();
const baseId = useId();

const TABS: { id: PeopleTab; label: string }[] = [
  { id: "friends", label: "Friends" },
  { id: "followers", label: "Followers" },
  { id: "following", label: "Following" },
  { id: "requests", label: "Requests" },
];

const lists = ref<Record<Exclude<PeopleTab, "requests">, Person[] | null>>({
  friends: null,
  followers: null,
  following: null,
});
const requests = ref<RequestsResponse | null>(null);
const loadError = ref("");
const actionError = ref("");
const busy = ref(false);
const confirming = ref<{ kind: "unfriend" | "remove" | "unfollow"; person: Person } | null>(null);
// Per-tab generation: a slow answer for an older load never overwrites a newer one.
const generations: Record<PeopleTab, number> = { friends: 0, followers: 0, following: 0, requests: 0 };

const incomingCount = computed(() =>
  requests.value ? requests.value.friend_incoming.length + requests.value.follow_incoming.length : 0,
);

async function load(tab: PeopleTab) {
  const mine = ++generations[tab];
  loadError.value = "";
  try {
    if (tab === "requests") {
      const r = await actions.listRequests();
      if (mine === generations[tab]) requests.value = r;
      return;
    }
    const fetcher = { friends: actions.listFriends, followers: actions.listFollowers, following: actions.listFollowing }[tab];
    const people = await fetcher();
    if (mine === generations[tab]) lists.value[tab] = people;
  } catch {
    if (mine === generations[tab]) loadError.value = "We couldn’t load this list.";
  }
}

onMounted(() => {
  void load(props.tab);
  // The Requests tab label shows how many wait for you.
  if (props.tab !== "requests") void load("requests");
});
watch(
  () => props.tab,
  (tab) => {
    confirming.value = null;
    actionError.value = "";
    void load(tab);
  },
);

async function run(action: (handle: string) => Promise<void>, person: Person) {
  busy.value = true;
  actionError.value = "";
  try {
    await action(person.handle);
    confirming.value = null;
    await load(props.tab);
    if (props.tab !== "requests") void load("requests");
  } catch (e) {
    actionError.value = socialErrorMessage(e);
  } finally {
    busy.value = false;
  }
}

const confirmCopy = computed(() => {
  const c = confirming.value;
  if (!c) return { message: "", button: "" };
  return {
    unfriend: { message: `Unfriend ${c.person.display_name}?`, button: "Unfriend" },
    remove: {
      message: `Remove ${c.person.display_name} from your followers? They won’t be told.`,
      button: "Remove",
    },
    unfollow: { message: `Unfollow ${c.person.display_name}?`, button: "Unfollow" },
  }[c.kind];
});

function confirmYes() {
  const c = confirming.value;
  if (!c) return;
  const op = { unfriend: actions.unfriend, remove: actions.removeFollower, unfollow: actions.unfollow }[c.kind];
  void run(op, c.person);
}

function select(tab: PeopleTab) {
  if (tab !== props.tab) emit("update:tab", tab);
}

// Arrow keys move between tabs (WAI-ARIA tabs pattern).
function onTabKey(e: KeyboardEvent, index: number) {
  const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
  if (!step) return;
  e.preventDefault();
  const next = TABS[(index + step + TABS.length) % TABS.length]!;
  select(next.id);
  document.getElementById(`${baseId}-tab-${next.id}`)?.focus();
}

const primary =
  "inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white";
const secondary =
  "inline-flex h-9 items-center rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800";
const empty = "rounded-xl bg-slate-100 p-4 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300";
const sectionTitle = "text-sm font-semibold text-slate-900 dark:text-slate-50";
</script>

<template>
  <div class="space-y-4">
    <div role="tablist" aria-label="Your people" class="flex gap-1 overflow-x-auto border-b border-slate-200 dark:border-slate-800">
      <button
        v-for="(t, i) in TABS"
        :id="`${baseId}-tab-${t.id}`"
        :key="t.id"
        type="button"
        role="tab"
        :aria-selected="tab === t.id ? 'true' : 'false'"
        :aria-controls="`${baseId}-panel`"
        :tabindex="tab === t.id ? 0 : -1"
        class="-mb-px inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-sm"
        :class="
          tab === t.id
            ? 'border-slate-900 font-medium text-slate-900 dark:border-slate-100 dark:text-slate-50'
            : 'border-transparent text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100'
        "
        :data-testid="`people-tab-${t.id}`"
        @click="select(t.id)"
        @keydown="onTabKey($event, i)"
      >
        {{ t.label }}
        <span
          v-if="t.id === 'requests' && incomingCount > 0"
          class="rounded-full bg-rose-600 px-1.5 text-xs font-medium text-white"
          data-testid="people-tab-requests-count"
        >{{ incomingCount }}<span class="sr-only"> waiting</span></span>
      </button>
    </div>

    <div :id="`${baseId}-panel`" role="tabpanel" :aria-labelledby="`${baseId}-tab-${tab}`" class="space-y-3">
      <p v-if="actionError" role="alert" class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100">
        {{ actionError }}
      </p>
      <div
        v-if="confirming"
        class="flex flex-wrap items-center gap-2 rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      >
        <span class="mr-auto" data-testid="confirm-message">{{ confirmCopy.message }}</span>
        <button
          type="button"
          class="inline-flex h-9 items-center rounded-md bg-rose-700 px-3 text-sm font-medium text-white hover:bg-rose-800 disabled:opacity-50 dark:bg-rose-600 dark:hover:bg-rose-500"
          :disabled="busy"
          data-testid="confirm-yes"
          @click="confirmYes"
        >
          {{ confirmCopy.button }}
        </button>
        <button type="button" :class="secondary" :disabled="busy" data-testid="confirm-keep" @click="confirming = null">
          Keep
        </button>
      </div>

      <div v-if="loadError" role="alert" class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100">
        {{ loadError }}
        <button type="button" class="font-medium underline" @click="load(tab)">Try again</button>
      </div>

      <!-- Friends / Followers / Following -->
      <template v-else-if="tab !== 'requests'">
        <p v-if="lists[tab] === null" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
        <p v-else-if="lists[tab]!.length === 0" :class="empty" :data-testid="`people-empty-${tab}`">
          <template v-if="tab === 'friends'">No friends yet. Find people above and send a friend request.</template>
          <template v-else-if="tab === 'followers'">No followers yet. People who ask to follow you appear under Requests.</template>
          <template v-else>You don’t follow anyone yet.</template>
        </p>
        <ul v-else class="space-y-2" :data-testid="`people-list-${tab}`">
          <PersonRow v-for="p in lists[tab]!" :key="p.handle" :person="p">
            <button
              v-if="tab === 'friends'"
              type="button"
              :class="secondary"
              :disabled="busy"
              data-testid="friend-remove"
              :aria-label="`Unfriend ${p.display_name}`"
              @click="confirming = { kind: 'unfriend', person: p }"
            >
              Unfriend
            </button>
            <button
              v-else-if="tab === 'followers'"
              type="button"
              :class="secondary"
              :disabled="busy"
              data-testid="follower-remove"
              :aria-label="`Remove follower ${p.display_name}`"
              @click="confirming = { kind: 'remove', person: p }"
            >
              Remove
            </button>
            <button
              v-else
              type="button"
              :class="secondary"
              :disabled="busy"
              data-testid="follow-remove"
              :aria-label="`Unfollow ${p.display_name}`"
              @click="confirming = { kind: 'unfollow', person: p }"
            >
              Unfollow
            </button>
          </PersonRow>
        </ul>
      </template>

      <!-- Requests -->
      <template v-else>
        <p v-if="requests === null" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
        <template v-else>
          <section class="space-y-2" data-testid="requests-friend-incoming">
            <h2 :class="sectionTitle">Friend requests</h2>
            <p v-if="requests.friend_incoming.length === 0" :class="empty">No friend requests.</p>
            <ul v-else class="space-y-2">
              <PersonRow v-for="p in requests.friend_incoming" :key="p.handle" :person="p">
                <button
type="button" :class="primary" :disabled="busy" data-testid="friend-accept"
                  :aria-label="`Accept friend request from ${p.display_name}`" @click="run(actions.acceptFriendRequest, p)">
                  Accept
                </button>
                <button
type="button" :class="secondary" :disabled="busy" data-testid="friend-decline"
                  :aria-label="`Decline friend request from ${p.display_name}`" @click="run(actions.declineFriendRequest, p)">
                  Decline
                </button>
              </PersonRow>
            </ul>
          </section>

          <section class="space-y-2" data-testid="requests-follow-incoming">
            <h2 :class="sectionTitle">Follow requests</h2>
            <p v-if="requests.follow_incoming.length === 0" :class="empty">No follow requests.</p>
            <ul v-else class="space-y-2">
              <PersonRow v-for="p in requests.follow_incoming" :key="p.handle" :person="p">
                <button
type="button" :class="primary" :disabled="busy" data-testid="follower-approve"
                  :aria-label="`Approve follow request from ${p.display_name}`" @click="run(actions.approveFollower, p)">
                  Approve
                </button>
                <button
type="button" :class="secondary" :disabled="busy" data-testid="follower-deny"
                  :aria-label="`Deny follow request from ${p.display_name}`" @click="run(actions.denyFollower, p)">
                  Deny
                </button>
              </PersonRow>
            </ul>
          </section>

          <section class="space-y-2" data-testid="requests-outgoing">
            <h2 :class="sectionTitle">Sent by you</h2>
            <p
              v-if="requests.friend_outgoing.length === 0 && requests.follow_outgoing.length === 0"
              :class="empty"
            >
              Nothing waiting on anyone else.
            </p>
            <ul v-else class="space-y-2">
              <PersonRow v-for="p in requests.friend_outgoing" :key="`friend-${p.handle}`" :person="p">
                <span class="text-xs text-slate-500 dark:text-slate-400">Friend request</span>
                <button
type="button" :class="secondary" :disabled="busy" data-testid="friend-cancel"
                  :aria-label="`Cancel friend request to ${p.display_name}`" @click="run(actions.cancelFriendRequest, p)">
                  Cancel
                </button>
              </PersonRow>
              <PersonRow v-for="p in requests.follow_outgoing" :key="`follow-${p.handle}`" :person="p">
                <span class="text-xs text-slate-500 dark:text-slate-400">Follow request</span>
                <button
type="button" :class="secondary" :disabled="busy" data-testid="follow-cancel"
                  :aria-label="`Cancel follow request to ${p.display_name}`" @click="run(actions.unfollow, p)">
                  Cancel
                </button>
              </PersonRow>
            </ul>
          </section>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { ViewerRelationship } from "~/types/api";
import { socialErrorMessage } from "~/utils/social";

/**
 * The friend and follow buttons on someone else's profile (ADR 0014). Shows
 * where things stand and the next step; the API enforces every rule. Emits
 * "changed" after a successful action so the page reloads the profile (a new
 * relationship can unlock or hide collections).
 */
const props = defineProps<{ handle: string; name: string; relationship: ViewerRelationship }>();
const emit = defineEmits<{ (e: "changed"): void }>();

const actions = useSocialActions();
const busy = ref(false);
const error = ref("");
const confirming = ref<"unfriend" | "unfollow" | null>(null);

async function run(action: (handle: string) => Promise<void>) {
  busy.value = true;
  error.value = "";
  try {
    await action(props.handle);
    confirming.value = null;
    emit("changed");
  } catch (e) {
    error.value = socialErrorMessage(e);
  } finally {
    busy.value = false;
  }
}

function confirmYes() {
  void run(confirming.value === "unfriend" ? actions.unfriend : actions.unfollow);
}

const primary =
  "inline-flex h-9 items-center gap-1.5 rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white";
const secondary =
  "inline-flex h-9 items-center gap-1.5 rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800";
const pill =
  "inline-flex h-9 items-center gap-1.5 rounded-md bg-emerald-50 px-3 text-sm font-medium text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200";
const pendingPill =
  "inline-flex h-9 items-center gap-1.5 rounded-md bg-amber-50 px-3 text-sm font-medium text-amber-800 dark:bg-amber-950 dark:text-amber-200";
const closed = "inline-flex h-9 items-center text-sm text-slate-500 dark:text-slate-400";
</script>

<template>
  <div class="mt-3 space-y-3" data-testid="relationship-actions">
    <div
      v-if="relationship.follows_you === 'requested'"
      class="flex flex-wrap items-center gap-2 rounded-lg bg-sky-50 p-3 text-sm text-sky-900 dark:bg-sky-950 dark:text-sky-100"
      data-testid="follow-request-banner"
    >
      <span class="mr-auto">{{ name }} asked to follow you.</span>
      <button type="button" :class="primary" :disabled="busy" data-testid="follower-approve" @click="run(actions.approveFollower)">
        Approve
      </button>
      <button type="button" :class="secondary" :disabled="busy" data-testid="follower-deny" @click="run(actions.denyFollower)">
        Deny
      </button>
    </div>

    <div
      v-if="relationship.friend === 'request_received'"
      class="flex flex-wrap items-center gap-2 rounded-lg bg-sky-50 p-3 text-sm text-sky-900 dark:bg-sky-950 dark:text-sky-100"
      data-testid="friend-request-banner"
    >
      <span class="mr-auto">{{ name }} wants to be friends.</span>
      <button type="button" :class="primary" :disabled="busy" data-testid="friend-accept" @click="run(actions.acceptFriendRequest)">
        Accept
      </button>
      <button type="button" :class="secondary" :disabled="busy" data-testid="friend-decline" @click="run(actions.declineFriendRequest)">
        Decline
      </button>
    </div>

    <div v-if="confirming" class="flex flex-wrap items-center gap-2 rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100">
      <span class="mr-auto" data-testid="confirm-message">
        {{ confirming === "unfriend" ? "Unfriend" : "Unfollow" }} {{ name }}?
      </span>
      <button
        type="button"
        class="inline-flex h-9 items-center rounded-md bg-rose-700 px-3 text-sm font-medium text-white hover:bg-rose-800 disabled:opacity-50 dark:bg-rose-600 dark:hover:bg-rose-500"
        :disabled="busy"
        data-testid="confirm-yes"
        @click="confirmYes"
      >
        {{ confirming === "unfriend" ? "Unfriend" : "Unfollow" }}
      </button>
      <button type="button" :class="secondary" :disabled="busy" data-testid="confirm-keep" @click="confirming = null">
        Keep
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <!-- Friendship -->
      <template v-if="relationship.friend === 'friends'">
        <span :class="pill" data-testid="friend-status">
          <Icon name="lucide:check" class="h-4 w-4" aria-hidden="true" />
          Friends
        </span>
        <button type="button" :class="secondary" :disabled="busy" data-testid="friend-remove" @click="confirming = 'unfriend'">
          Unfriend
        </button>
      </template>
      <template v-else-if="relationship.friend === 'request_sent'">
        <span :class="pendingPill" data-testid="friend-status">Friend request sent</span>
        <button type="button" :class="secondary" :disabled="busy" data-testid="friend-cancel" @click="run(actions.cancelFriendRequest)">
          Cancel request
        </button>
      </template>
      <template v-else-if="relationship.friend === 'none'">
        <button
          v-if="relationship.accepts_friend_requests"
          type="button"
          :class="primary"
          :disabled="busy"
          data-testid="friend-add"
          @click="run(actions.sendFriendRequest)"
        >
          <Icon name="lucide:user-plus" class="h-4 w-4" aria-hidden="true" />
          Add friend
        </button>
        <span v-else :class="closed" data-testid="friend-closed">Not accepting friend requests</span>
      </template>

      <!-- Following (you → them) -->
      <template v-if="relationship.following === 'following'">
        <span :class="pill" data-testid="follow-status">
          <Icon name="lucide:check" class="h-4 w-4" aria-hidden="true" />
          Following
        </span>
        <button type="button" :class="secondary" :disabled="busy" data-testid="follow-remove" @click="confirming = 'unfollow'">
          Unfollow
        </button>
      </template>
      <template v-else-if="relationship.following === 'requested'">
        <span :class="pendingPill" data-testid="follow-status">Follow requested</span>
        <button type="button" :class="secondary" :disabled="busy" data-testid="follow-cancel" @click="run(actions.unfollow)">
          Cancel request
        </button>
      </template>
      <template v-else>
        <button
          v-if="relationship.accepts_follow_requests"
          type="button"
          :class="secondary"
          :disabled="busy"
          data-testid="follow-request"
          @click="run(actions.requestFollow)"
        >
          Request to follow
        </button>
        <span v-else :class="closed" data-testid="follow-closed">Not accepting follow requests</span>
      </template>

      <span
        v-if="relationship.follows_you === 'following'"
        class="text-xs text-slate-500 dark:text-slate-400"
        data-testid="follows-you"
      >Follows you</span>
    </div>

    <p v-if="error" role="alert" class="text-sm text-rose-700 dark:text-rose-300">{{ error }}</p>
  </div>
</template>

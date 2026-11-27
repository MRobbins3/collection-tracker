<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from "vue";
import UserAvatar from "./UserAvatar.vue";
import { useDebouncedRef } from "~/composables/useDebouncedRef";
import type { CollectionVisibility, FieldErrors, HandleAvailability, Profile } from "~/types/api";
import { VISIBILITY_OPTIONS } from "~/utils/privacy";
import { collapseWhitespace, handleProblem, normalizeHandle, suggestTitleCase } from "~/utils/text";

/**
 * Settings form: profile (display name, handle, bio) and privacy (who sees
 * your collections, which requests you accept). The API validates and
 * normalizes everything again; the checks here are for instant feedback.
 */
const props = defineProps<{ profile: Profile }>();
const emit = defineEmits<{ (e: "saved", profile: Profile): void }>();

const { update, checkHandle } = useProfileActions();

const BIO_MAX = 160;
const HANDLE_CHECK_MS = 300;

const displayName = ref(props.profile.display_name);
const handle = ref(props.profile.handle);
const bio = ref(props.profile.bio);
const visibility = ref<CollectionVisibility>(props.profile.collection_visibility);
const acceptFriends = ref(props.profile.accept_friend_requests);
const acceptFollows = ref(props.profile.accept_follow_requests);

const saving = ref(false);
const saved = ref(false);
const generalError = ref<string | null>(null);
const fieldErrors = ref<Record<string, string>>({});

const ids = {
  name: useId(),
  nameHelp: useId(),
  handle: useId(),
  handleStatus: useId(),
  bio: useId(),
  bioHelp: useId(),
};

const nameSuggestion = computed(() => suggestTitleCase(displayName.value));
function applySuggestion() {
  if (nameSuggestion.value) displayName.value = nameSuggestion.value;
}

// --- handle availability --------------------------------------------------
const currentHandle = ref(props.profile.handle);
const normalizedHandle = computed(() => normalizeHandle(handle.value));
const formatProblem = computed(() => handleProblem(normalizedHandle.value));
const availability = ref<HandleAvailability | null>(null);
const checking = ref(false);
const debouncedHandle = useDebouncedRef(normalizedHandle, HANDLE_CHECK_MS);

watch(debouncedHandle, async (h) => {
  availability.value = null;
  if (h === currentHandle.value || handleProblem(h)) return;
  checking.value = true;
  try {
    const res = await checkHandle(h);
    if (res.handle === normalizedHandle.value) availability.value = res;
  } catch (e) {
    // The save reports any real problem; the live check is a convenience.
    // Only "slow down" (429) is worth showing.
    const err = e as { statusCode?: number; data?: { error?: string } };
    if (err?.statusCode === 429 && h === normalizedHandle.value) {
      availability.value = {
        handle: h,
        available: false,
        message: err.data?.error ?? "You’re checking handles too quickly. Wait a moment and try again.",
      };
    }
  } finally {
    checking.value = false;
  }
});

const handleStatus = computed<{ text: string; tone: "ok" | "error" | "muted" } | null>(() => {
  if (fieldErrors.value.handle) return { text: fieldErrors.value.handle, tone: "error" };
  if (formatProblem.value) return { text: formatProblem.value, tone: "error" };
  if (normalizedHandle.value === currentHandle.value) return null;
  const a = availability.value;
  if (a && a.handle === normalizedHandle.value) {
    return a.available
      ? { text: `@${a.handle} is available.`, tone: "ok" }
      : { text: a.message, tone: "error" };
  }
  return checking.value || normalizedHandle.value !== debouncedHandle.value
    ? { text: "Checking…", tone: "muted" }
    : null;
});
const handleInvalid = computed(() => handleStatus.value?.tone === "error");

// Editing a field clears the server's message for it.
watch(displayName, () => clearField("display_name"));
watch(handle, () => clearField("handle"));
watch(bio, () => clearField("bio"));
watch(visibility, () => clearField("collection_visibility"));
function clearField(name: string) {
  saved.value = false;
  if (fieldErrors.value[name]) {
    const { [name]: _gone, ...rest } = fieldErrors.value;
    fieldErrors.value = rest;
  }
}

const bioLength = computed(() => [...bio.value].length);
const canSave = computed(() => !saving.value && !formatProblem.value);

async function onSubmit() {
  if (!canSave.value) return;
  saving.value = true;
  saved.value = false;
  generalError.value = null;
  fieldErrors.value = {};
  try {
    const profile = await update({
      handle: normalizedHandle.value,
      display_name: collapseWhitespace(displayName.value),
      bio: collapseWhitespace(bio.value),
      collection_visibility: visibility.value,
      accept_friend_requests: acceptFriends.value,
      accept_follow_requests: acceptFollows.value,
    });
    displayName.value = profile.display_name;
    handle.value = profile.handle;
    bio.value = profile.bio;
    currentHandle.value = profile.handle;
    availability.value = null;
    // Let the field watchers see the normalized values first, or they'd
    // clear "Saved." straight away.
    await nextTick();
    saved.value = true;
    emit("saved", profile);
  } catch (e) {
    const body = (e as { data?: FieldErrors })?.data;
    if (body?.fields && Object.keys(body.fields).length > 0) {
      fieldErrors.value = body.fields;
    } else {
      generalError.value = "We couldn’t save your changes. Try again.";
    }
  } finally {
    saving.value = false;
  }
}

const inputClass =
  "mt-1 block h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-base text-slate-900 shadow-sm focus:border-slate-500 focus:outline-none focus:ring-2 focus:ring-slate-300 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-50 dark:focus:ring-slate-600";
const errorText = "mt-1 text-sm text-rose-700 dark:text-rose-300";
const helpText = "mt-1 text-xs text-slate-500 dark:text-slate-400";
</script>

<template>
  <form class="space-y-6" novalidate data-testid="profile-form" @submit.prevent="onSubmit">
    <section
      class="space-y-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
      aria-labelledby="profile-section-heading"
    >
      <div class="flex items-center gap-3">
        <UserAvatar :name="displayName || profile.display_name" :color="profile.avatar_color" size="lg" />
        <div class="min-w-0">
          <h2 id="profile-section-heading" class="text-base font-semibold text-slate-900 dark:text-slate-50">
            Profile
          </h2>
          <p class="text-sm text-slate-600 dark:text-slate-400">
            Your avatar uses your initials. Everyone who can find your profile sees these.
          </p>
        </div>
      </div>

      <div>
        <label :for="ids.name" class="block text-sm font-medium text-slate-700 dark:text-slate-200">
          Display name
        </label>
        <input
          :id="ids.name"
          v-model="displayName"
          type="text"
          autocomplete="name"
          maxlength="50"
          :class="inputClass"
          :aria-invalid="fieldErrors.display_name ? 'true' : 'false'"
          :aria-describedby="ids.nameHelp"
          data-testid="profile-display-name"
        >
        <div :id="ids.nameHelp">
          <p v-if="fieldErrors.display_name" :class="errorText" data-testid="profile-display-name-error">
            {{ fieldErrors.display_name }}
          </p>
          <p v-else-if="nameSuggestion" :class="helpText">
            Tip: names usually look best in Title Case.
            <button
              type="button"
              class="font-medium text-slate-900 underline underline-offset-2 dark:text-slate-100"
              data-testid="display-name-suggestion"
              @click="applySuggestion"
            >
              Use “{{ nameSuggestion }}”
            </button>
          </p>
        </div>
      </div>

      <div>
        <label :for="ids.handle" class="block text-sm font-medium text-slate-700 dark:text-slate-200">
          Handle
        </label>
        <div class="relative">
          <span
            class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-base text-slate-500 dark:text-slate-400"
            aria-hidden="true"
          >@</span>
          <input
            :id="ids.handle"
            v-model="handle"
            type="text"
            autocapitalize="none"
            autocomplete="username"
            spellcheck="false"
            maxlength="31"
            :class="[inputClass, 'pl-7']"
            :aria-invalid="handleInvalid ? 'true' : 'false'"
            :aria-describedby="ids.handleStatus"
            data-testid="profile-handle"
          >
        </div>
        <div :id="ids.handleStatus" aria-live="polite">
          <p
            v-if="handleStatus"
            data-testid="handle-status"
            :class="
              handleStatus.tone === 'error'
                ? errorText
                : handleStatus.tone === 'ok'
                  ? 'mt-1 text-sm text-emerald-700 dark:text-emerald-300'
                  : helpText
            "
          >
            <span v-if="fieldErrors.handle" data-testid="profile-handle-error">{{ handleStatus.text }}</span>
            <template v-else>{{ handleStatus.text }}</template>
          </p>
          <p v-else :class="helpText">
            Your profile link: /u/{{ normalizedHandle || "…" }}. Lowercase letters, numbers, and underscores.
          </p>
        </div>
      </div>

      <div>
        <label :for="ids.bio" class="block text-sm font-medium text-slate-700 dark:text-slate-200">
          Bio <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
        </label>
        <textarea
          :id="ids.bio"
          v-model="bio"
          rows="2"
          :maxlength="BIO_MAX"
          class="mt-1 block w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-base text-slate-900 shadow-sm focus:border-slate-500 focus:outline-none focus:ring-2 focus:ring-slate-300 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-50 dark:focus:ring-slate-600"
          :aria-invalid="fieldErrors.bio ? 'true' : 'false'"
          :aria-describedby="ids.bioHelp"
          data-testid="profile-bio"
        />
        <div :id="ids.bioHelp" class="flex justify-between gap-3">
          <p v-if="fieldErrors.bio" :class="errorText" data-testid="profile-bio-error">{{ fieldErrors.bio }}</p>
          <p v-else :class="helpText">What you collect, in a sentence.</p>
          <p :class="helpText" data-testid="profile-bio-count">{{ bioLength }}/{{ BIO_MAX }}</p>
        </div>
      </div>
    </section>

    <section
      class="space-y-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
      aria-labelledby="privacy-section-heading"
    >
      <h2 id="privacy-section-heading" class="text-base font-semibold text-slate-900 dark:text-slate-50">
        Privacy
      </h2>
      <fieldset data-testid="privacy-fieldset">
        <legend class="text-sm font-medium text-slate-700 dark:text-slate-200">Who can see your collections?</legend>
        <div class="mt-2 space-y-2">
          <label
            v-for="o in VISIBILITY_OPTIONS"
            :key="o.value"
            class="flex cursor-pointer items-start gap-3 rounded-lg p-3 ring-1 ring-slate-200 has-[:checked]:bg-slate-50 has-[:checked]:ring-slate-500 dark:ring-slate-700 dark:has-[:checked]:bg-slate-800 dark:has-[:checked]:ring-slate-400"
          >
            <input
              v-model="visibility"
              type="radio"
              name="collection_visibility"
              :value="o.value"
              class="mt-1 h-4 w-4 accent-slate-900 dark:accent-slate-100"
              :data-testid="`visibility-${o.value.replace(/_/g, '-')}`"
            >
            <span>
              <span class="block text-sm font-medium text-slate-900 dark:text-slate-50">{{ o.label }}</span>
              <span class="block text-xs text-slate-600 dark:text-slate-400">{{ o.description }}</span>
            </span>
          </label>
        </div>
        <p v-if="fieldErrors.collection_visibility" :class="errorText">{{ fieldErrors.collection_visibility }}</p>
      </fieldset>

      <fieldset class="space-y-2">
        <legend class="text-sm font-medium text-slate-700 dark:text-slate-200">Requests</legend>
        <label class="flex items-start gap-3 text-sm text-slate-700 dark:text-slate-200">
          <input
            v-model="acceptFriends"
            type="checkbox"
            class="mt-0.5 h-4 w-4 accent-slate-900 dark:accent-slate-100"
            data-testid="accept-friend-requests"
          >
          <span>Let people send me friend requests</span>
        </label>
        <label class="flex items-start gap-3 text-sm text-slate-700 dark:text-slate-200">
          <input
            v-model="acceptFollows"
            type="checkbox"
            class="mt-0.5 h-4 w-4 accent-slate-900 dark:accent-slate-100"
            data-testid="accept-follow-requests"
          >
          <span>Let people ask to follow me <span class="text-slate-500 dark:text-slate-400">(you approve each one)</span></span>
        </label>
      </fieldset>
    </section>

    <div class="flex flex-wrap items-center gap-3">
      <button
        type="submit"
        class="inline-flex h-11 items-center rounded-md bg-slate-900 px-5 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        :disabled="!canSave"
        data-testid="profile-save"
      >
        {{ saving ? "Saving…" : "Save changes" }}
      </button>
      <p role="status" class="text-sm">
        <span v-if="saved" class="text-emerald-700 dark:text-emerald-300" data-testid="profile-saved">Saved.</span>
      </p>
      <p v-if="generalError" role="alert" :class="errorText" data-testid="profile-error">{{ generalError }}</p>
      <p v-else-if="Object.keys(fieldErrors).length > 0" role="alert" :class="errorText">
        Please fix the highlighted fields.
      </p>
    </div>
  </form>
</template>

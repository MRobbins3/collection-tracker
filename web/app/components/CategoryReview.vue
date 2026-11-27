<script setup lang="ts">
import { computed, ref, useId } from "vue";
import AttributeFields from "./AttributeFields.vue";
import { apiErrorMessage, statusLabel } from "~/utils/moderation";
import type { AttributeSchema, ModerationRequest } from "~/types/api";

/**
 * A user-built category asking to join the public list (ADR 0013), for an
 * admin: what it tracks, a live preview of its item form, and approve or
 * reject with a reason the creator sees. Categories are approved as built;
 * to ask for changes, reject with a reason.
 */
const props = defineProps<{ request: ModerationRequest }>();
const emit = defineEmits<{
  (e: "decided", request: ModerationRequest): void;
  (e: "reload"): void;
}>();

const { approve, reject } = useModerationAdmin();
const reasonId = useId();
const rejecting = ref(false);
const reason = ref("");
const busy = ref(false);
const error = ref<string | null>(null);
const conflict = ref(false);
const preview = ref<Record<string, unknown>>({});

const category = computed(() => props.request.category);
const schema = computed(() => category.value?.attribute_schema as AttributeSchema | undefined);
const fields = computed(() =>
  Object.entries(schema.value?.properties ?? {}).map(([key, p]) => ({
    key,
    title: p.title ?? key,
    kind: p.type === "string" ? (p.enum?.length ? "Pick list" : "Text") : "Number",
    options: p.enum ?? [],
    identifies: category.value?.catalog_identifier === key,
  })),
);
const isPending = computed(() => props.request.status === "pending");
const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

async function run(action: () => Promise<ModerationRequest>) {
  busy.value = true;
  error.value = null;
  conflict.value = false;
  try {
    const updated = await action();
    rejecting.value = false;
    emit("decided", updated);
  } catch (e) {
    console.error(e);
    const status = (e as { statusCode?: number; status?: number })?.statusCode ?? (e as { status?: number })?.status;
    conflict.value = status === 409;
    error.value = apiErrorMessage(e, "That didn’t work. Please try again.");
  } finally {
    busy.value = false;
  }
}

function onReject() {
  if (!reason.value.trim()) {
    error.value = "Say why, so the person who built it knows what to change.";
    return;
  }
  void run(() => reject(props.request.id, reason.value.trim()));
}

const primaryBtn =
  "inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white";
const secondaryBtn =
  "inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800";
const cardClass = "rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800";
</script>

<template>
  <article class="space-y-4" data-testid="category-review">
    <header>
      <p class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">New category</p>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50" data-testid="review-name">
        {{ category?.name ?? request.category_name }}
      </h1>
      <p v-if="category?.description" class="mt-1 text-slate-600 dark:text-slate-400">{{ category.description }}</p>
      <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        From {{ request.submitter.display_name }} (@{{ request.submitter.handle }}) ·
        {{ dateFormat.format(new Date(request.created_at)) }}
      </p>
      <p
        class="mt-2 inline-block rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-800 dark:bg-slate-800 dark:text-slate-100"
        data-testid="review-status"
      >
        {{ statusLabel(request.status, request.kind) }}
      </p>
    </header>

    <p v-if="request.note" class="rounded-md bg-slate-50 p-3 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200">
      <span class="font-medium">Their note:</span> {{ request.note }}
    </p>

    <section :class="cardClass">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">What it tracks</h2>
      <ul class="mt-3 space-y-2 text-sm" data-testid="review-category-fields">
        <li v-for="f in fields" :key="f.key">
          <span class="font-medium text-slate-900 dark:text-slate-100">{{ f.title }}</span>
          <span class="text-slate-500 dark:text-slate-400"> · {{ f.kind }}</span>
          <span v-if="f.identifies" class="text-slate-500 dark:text-slate-400"> · tells items apart</span>
          <p v-if="f.options.length > 0" class="text-xs text-slate-600 dark:text-slate-400">{{ f.options.join(", ") }}</p>
        </li>
      </ul>
    </section>

    <section v-if="schema" :class="cardClass" class="space-y-2">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Item form preview</h2>
      <AttributeFields :schema="schema" :model-value="preview" @update:model-value="preview = $event" />
    </section>

    <section
      v-if="!isPending"
      class="rounded-xl bg-slate-50 p-4 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
      data-testid="review-outcome"
    >
      <p>
        {{ statusLabel(request.status, request.kind) }}<template v-if="request.decided_by">
          by {{ request.decided_by.display_name }}</template>
      </p>
      <p v-if="request.reason" class="mt-1"><span class="font-medium">Reason:</span> {{ request.reason }}</p>
      <NuxtLink
        v-if="request.status === 'approved'"
        :to="`/categories/${request.category_slug}`"
        class="mt-1 inline-block underline"
      >
        Open the category
      </NuxtLink>
    </section>

    <section v-else class="space-y-3" data-testid="review-actions">
      <div v-if="!rejecting" class="flex flex-wrap gap-2">
        <button type="button" :class="primaryBtn" :disabled="busy" data-testid="review-approve" @click="run(() => approve(request.id))">
          Approve for the public list
        </button>
        <button type="button" :class="secondaryBtn" :disabled="busy" data-testid="review-reject" @click="rejecting = true">
          Reject
        </button>
      </div>
      <form v-else :class="cardClass" class="space-y-3" @submit.prevent="onReject">
        <div>
          <label :for="reasonId" class="block text-sm font-medium text-slate-900 dark:text-slate-100">Why are you rejecting it?</label>
          <p class="text-xs text-slate-500 dark:text-slate-400">The person who built it sees this. Their category stays private to them.</p>
          <textarea
            :id="reasonId"
            v-model="reason"
            rows="3"
            maxlength="500"
            class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
            data-testid="review-reject-reason"
          />
        </div>
        <div class="flex gap-2">
          <button type="submit" :class="primaryBtn" :disabled="busy" data-testid="review-reject-submit">Reject</button>
          <button type="button" :class="secondaryBtn" @click="rejecting = false">Cancel</button>
        </div>
      </form>
      <div v-if="error" role="alert" class="text-sm text-rose-700 dark:text-rose-300" data-testid="review-error">
        {{ error }}
        <button v-if="conflict" type="button" class="ml-1 underline" @click="emit('reload')">Reload</button>
      </div>
    </section>
  </article>
</template>

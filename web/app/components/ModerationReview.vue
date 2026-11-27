<script setup lang="ts">
import { computed, ref, useId, watch } from "vue";
import AttributeFields from "./AttributeFields.vue";
import { catalogEntryPath, humanizeKey } from "~/utils/catalog";
import { apiErrorMessage, diffRows, formatValue, kindLabel, statusLabel } from "~/utils/moderation";
import type { AttributeSchema, CatalogEntry, EntryFields, ModerationRequest } from "~/types/api";

/**
 * One moderation request, for an admin (ADR 0012): what was submitted (or,
 * for a correction, a before/after diff), and the actions: approve,
 * edit then approve, reject with a reason the submitter sees, and for a
 * duplicate suggestion, merge into an existing catalog entry.
 */
const props = defineProps<{
  request: ModerationRequest;
  schema?: AttributeSchema;
  /** Human label of the category's identifier attribute ("Set number"), if any. */
  identifierTitle?: string | null;
}>();

const emit = defineEmits<{
  (e: "decided", request: ModerationRequest): void;
  (e: "reload"): void;
}>();

const { approve, reject, merge } = useModerationAdmin();
const { search } = useCatalogSearch();

const ids = { name: useId(), description: useId(), reason: useId(), mergeSearch: useId(), mergeReason: useId() };
type Mode = "idle" | "edit" | "reject" | "merge";
const mode = ref<Mode>("idle");
const busy = ref(false);
const error = ref<string | null>(null);
const conflict = ref(false);

const isPending = computed(() => props.request.status === "pending");
const isCorrection = computed(() => props.request.kind === "catalog_correction");
const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

/** The entry as it would be after approval: submitted values, plus a correction's changes. */
const proposedEntry = computed<Pick<CatalogEntry, "name" | "description" | "attributes">>(() => {
  const e = props.request.entry;
  const p = props.request.proposed ?? {};
  return {
    name: p.name ?? e.name,
    description: p.description !== undefined ? p.description : e.description,
    attributes: p.attributes ?? e.attributes,
  };
});

/** Labelled rows for a submitted entry's details, in schema order. */
const submittedRows = computed(() => {
  const props_ = props.schema?.properties ?? {};
  const attrs = props.request.entry.attributes ?? {};
  const keys = [...Object.keys(props_), ...Object.keys(attrs).filter((k) => !(k in props_))];
  return keys
    .filter((k) => attrs[k] !== undefined && attrs[k] !== "")
    .map((k) => ({ key: k, label: props_[k]?.title ?? humanizeKey(k), value: formatValue(attrs[k]) }));
});

const correctionDiff = computed(() => {
  if (!isCorrection.value) return [];
  if (isPending.value) return diffRows(props.schema, props.request.entry, props.request.proposed ?? {});
  const prev = props.request.previous;
  const applied = props.request.applied;
  if (!prev || !applied) return [];
  return diffRows(
    props.schema,
    { name: prev.name ?? "", description: prev.description, attributes: prev.attributes ?? {} },
    applied,
  );
});

// ---- edit then approve ------------------------------------------------------
const editName = ref("");
const editDescription = ref("");
const editAttrs = ref<Record<string, unknown>>({});

function startEdit() {
  const p = proposedEntry.value;
  editName.value = p.name;
  editDescription.value = p.description ?? "";
  editAttrs.value = { ...(p.attributes ?? {}) };
  setMode("edit");
}

// ---- reject / merge ---------------------------------------------------------
const reason = ref("");
const mergeQuery = ref("");
const mergeResults = ref<CatalogEntry[]>([]);
const mergeTarget = ref<CatalogEntry | null>(null);
const mergeReason = ref("");
let mergeGeneration = 0;

watch(mergeQuery, async (q) => {
  const gen = ++mergeGeneration;
  const trimmed = q.trim();
  if (!trimmed) {
    mergeResults.value = [];
    return;
  }
  try {
    const page = await search(props.request.category_slug, trimmed);
    if (gen === mergeGeneration) mergeResults.value = page.entries;
  } catch (e) {
    console.error(e);
    if (gen === mergeGeneration) mergeResults.value = [];
  }
});

function startMerge() {
  mergeQuery.value = props.request.entry.name;
  mergeTarget.value = null;
  setMode("merge");
}

function setMode(m: Mode) {
  mode.value = m;
  error.value = null;
}

async function run(action: () => Promise<ModerationRequest>) {
  busy.value = true;
  error.value = null;
  conflict.value = false;
  try {
    const updated = await action();
    mode.value = "idle";
    emit("decided", updated);
  } catch (e) {
    console.error(e);
    conflict.value = (e as { status?: number; statusCode?: number })?.status === 409
      || (e as { statusCode?: number })?.statusCode === 409;
    error.value = apiErrorMessage(e, "That didn’t work. Please try again.");
  } finally {
    busy.value = false;
  }
}

function onApprove() {
  void run(() => approve(props.request.id));
}

function onApproveEdited() {
  const name = editName.value.trim();
  if (name.length < 1 || name.length > 200) {
    error.value = "Name must be 1–200 characters.";
    return;
  }
  const edits: EntryFields = {
    name,
    description: editDescription.value.trim(),
    attributes: Object.fromEntries(Object.entries(editAttrs.value).filter(([, v]) => v !== undefined && v !== "")),
  };
  void run(() => approve(props.request.id, edits));
}

function onReject() {
  if (!reason.value.trim()) {
    error.value = "Say why, so the person who sent it knows what to fix.";
    return;
  }
  void run(() => reject(props.request.id, reason.value.trim()));
}

function onMerge() {
  if (!mergeTarget.value) {
    error.value = "Pick the catalog entry it duplicates.";
    return;
  }
  const target = mergeTarget.value;
  void run(() => merge(props.request.id, target.id, mergeReason.value.trim() || undefined));
}

const inputClass =
  "mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400";
const primaryBtn =
  "inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white";
const secondaryBtn =
  "inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800";
</script>

<template>
  <article class="space-y-4" data-testid="moderation-review">
    <header>
      <p class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
        {{ kindLabel(request.kind) }} · {{ request.category_name }}
      </p>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50" data-testid="review-name">
        {{ request.entry.name }}
      </h1>
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

    <p
      v-if="request.note"
      class="rounded-md bg-slate-50 p-3 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
    >
      <span class="font-medium">Their note:</span> {{ request.note }}
    </p>

    <!-- What was submitted -->
    <section
      v-if="!isCorrection"
      class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    >
      <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Suggested entry</h2>
      <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-sm" data-testid="review-submitted">
        <dt class="text-slate-500 dark:text-slate-400">Name</dt>
        <dd class="text-slate-900 dark:text-slate-100">{{ request.entry.name }}</dd>
        <template v-if="request.entry.description">
          <dt class="text-slate-500 dark:text-slate-400">Description</dt>
          <dd class="text-slate-900 dark:text-slate-100">{{ request.entry.description }}</dd>
        </template>
        <template v-for="row in submittedRows" :key="row.key">
          <dt class="text-slate-500 dark:text-slate-400">{{ row.label }}</dt>
          <dd class="text-slate-900 dark:text-slate-100">{{ row.value }}</dd>
        </template>
      </dl>
      <p v-if="request.previous" class="mt-3 text-xs text-slate-500 dark:text-slate-400">
        Edited by the reviewer before approval.
      </p>
    </section>

    <section
      v-else
      class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    >
      <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        {{ isPending ? "Proposed changes" : "Changes made" }}
      </h2>
      <table v-if="correctionDiff.length > 0" class="mt-3 w-full text-left text-sm" data-testid="review-diff">
        <thead>
          <tr class="text-xs text-slate-500 dark:text-slate-400">
            <th scope="col" class="py-1 pr-2 font-medium">Detail</th>
            <th scope="col" class="py-1 pr-2 font-medium">Now</th>
            <th scope="col" class="py-1 font-medium">Proposed</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in correctionDiff"
            :key="row.key"
            class="border-t border-slate-100 align-top dark:border-slate-800"
            data-testid="review-diff-row"
          >
            <th scope="row" class="py-1.5 pr-2 font-medium text-slate-700 dark:text-slate-200">{{ row.label }}</th>
            <td class="py-1.5 pr-2 text-rose-800 line-through decoration-rose-400 dark:text-rose-300">{{ row.before }}</td>
            <td class="py-1.5 text-emerald-800 dark:text-emerald-300">{{ row.after }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="mt-3 text-sm text-slate-600 dark:text-slate-400">
        The entry already matches this correction.
      </p>
      <NuxtLink
        :to="catalogEntryPath(request.category_slug, request.entry.id)"
        class="mt-3 inline-block text-xs text-slate-600 underline dark:text-slate-300"
      >
        Open the catalog entry
      </NuxtLink>
    </section>

    <!-- Outcome -->
    <section
      v-if="!isPending"
      class="rounded-xl bg-slate-50 p-4 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
      data-testid="review-outcome"
    >
      <p>
        {{ statusLabel(request.status, request.kind) }}<template v-if="request.decided_by">
          by {{ request.decided_by.display_name }}</template><template v-if="request.decided_at">
          · {{ dateFormat.format(new Date(request.decided_at)) }}</template>
      </p>
      <p v-if="request.reason" class="mt-1"><span class="font-medium">Reason:</span> {{ request.reason }}</p>
      <p v-if="request.merged_into" class="mt-1">
        Merged into
        <NuxtLink :to="catalogEntryPath(request.category_slug, request.merged_into.id)" class="underline">
          {{ request.merged_into.name }}
        </NuxtLink>
      </p>
      <p v-else-if="request.status === 'approved' && !isCorrection" class="mt-1">
        <NuxtLink :to="catalogEntryPath(request.category_slug, request.entry.id)" class="underline">
          View it in the catalog
        </NuxtLink>
      </p>
    </section>

    <!-- Actions -->
    <section v-if="isPending" class="space-y-3" data-testid="review-actions">
      <div v-if="mode === 'idle'" class="flex flex-wrap gap-2">
        <button type="button" :class="primaryBtn" :disabled="busy" data-testid="review-approve" @click="onApprove">
          Approve
        </button>
        <button type="button" :class="secondaryBtn" :disabled="busy" data-testid="review-edit" @click="startEdit">
          Edit, then approve
        </button>
        <button
          v-if="!isCorrection"
          type="button"
          :class="secondaryBtn"
          :disabled="busy"
          data-testid="review-merge"
          @click="startMerge"
        >
          Merge into existing
        </button>
        <button
          type="button"
          :class="secondaryBtn"
          :disabled="busy"
          data-testid="review-reject"
          @click="setMode('reject')"
        >
          Reject
        </button>
      </div>

      <form
        v-else-if="mode === 'edit'"
        class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
        @submit.prevent="onApproveEdited"
      >
        <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Edit, then approve</h2>
        <div>
          <label :for="ids.name" class="block text-xs font-medium text-slate-700 dark:text-slate-300">Name</label>
          <input :id="ids.name" v-model="editName" type="text" maxlength="200" :class="inputClass" data-testid="review-edit-name">
        </div>
        <AttributeFields
          v-if="schema"
          :schema="schema"
          :model-value="editAttrs"
          @update:model-value="editAttrs = $event"
        />
        <div>
          <label :for="ids.description" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
            Description
          </label>
          <textarea :id="ids.description" v-model="editDescription" rows="2" maxlength="1000" :class="inputClass" />
        </div>
        <div class="flex gap-2">
          <button type="submit" :class="primaryBtn" :disabled="busy" data-testid="review-edit-submit">
            Approve with these changes
          </button>
          <button type="button" :class="secondaryBtn" @click="setMode('idle')">Cancel</button>
        </div>
      </form>

      <form
        v-else-if="mode === 'reject'"
        class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
        @submit.prevent="onReject"
      >
        <div>
          <label :for="ids.reason" class="block text-sm font-medium text-slate-900 dark:text-slate-100">
            Why are you rejecting it?
          </label>
          <p class="text-xs text-slate-500 dark:text-slate-400">The person who sent it sees this.</p>
          <textarea
            :id="ids.reason"
            v-model="reason"
            rows="3"
            maxlength="500"
            :class="inputClass"
            data-testid="review-reject-reason"
          />
        </div>
        <div class="flex gap-2">
          <button type="submit" :class="primaryBtn" :disabled="busy" data-testid="review-reject-submit">Reject</button>
          <button type="button" :class="secondaryBtn" @click="setMode('idle')">Cancel</button>
        </div>
      </form>

      <form
        v-else-if="mode === 'merge'"
        class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
        @submit.prevent="onMerge"
      >
        <div>
          <label :for="ids.mergeSearch" class="block text-sm font-medium text-slate-900 dark:text-slate-100">
            Which catalog entry is it a duplicate of?
          </label>
          <input
            :id="ids.mergeSearch"
            v-model="mergeQuery"
            type="search"
            autocomplete="off"
            :class="inputClass"
            data-testid="review-merge-search"
          >
        </div>
        <ul v-if="mergeResults.length > 0" class="space-y-1" aria-label="Catalog matches">
          <li v-for="e in mergeResults" :key="e.id">
            <label
              class="flex cursor-pointer items-center gap-2 rounded-md border border-slate-200 px-3 py-2 text-sm dark:border-slate-700"
              data-testid="review-merge-option"
            >
              <input v-model="mergeTarget" type="radio" name="merge-target" :value="e">
              <span class="text-slate-900 dark:text-slate-100">{{ e.name }}</span>
              <span v-if="identifierTitle && e.identifier" class="text-xs text-slate-500 dark:text-slate-400">
                {{ identifierTitle }} {{ e.identifier }}
              </span>
            </label>
          </li>
        </ul>
        <p v-else-if="mergeQuery.trim()" class="text-xs text-slate-500 dark:text-slate-400">No matching entries.</p>
        <div>
          <label :for="ids.mergeReason" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
            Note for the person who sent it <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
          </label>
          <input :id="ids.mergeReason" v-model="mergeReason" type="text" maxlength="500" :class="inputClass">
        </div>
        <div class="flex gap-2">
          <button type="submit" :class="primaryBtn" :disabled="busy" data-testid="review-merge-submit">Merge</button>
          <button type="button" :class="secondaryBtn" @click="setMode('idle')">Cancel</button>
        </div>
      </form>

      <div v-if="error" role="alert" class="text-sm text-rose-700 dark:text-rose-300" data-testid="review-error">
        {{ error }}
        <button v-if="conflict" type="button" class="ml-1 underline" @click="emit('reload')">Reload</button>
      </div>
    </section>
  </article>
</template>

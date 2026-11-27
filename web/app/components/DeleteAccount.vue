<script setup lang="ts">
import { computed, nextTick, ref, useId } from "vue";
import { normalizeHandle } from "~/utils/text";

/**
 * Danger zone: delete the account and everything in it. The user must type
 * their handle to confirm; the API checks it again.
 */
const props = defineProps<{ handle: string }>();
const emit = defineEmits<{ (e: "deleted"): void }>();

const { deleteAccount } = useProfileActions();

const confirming = ref(false);
const typed = ref("");
const deleting = ref(false);
const error = ref<string | null>(null);
const inputId = useId();
const input = ref<HTMLInputElement | null>(null);

const matches = computed(() => normalizeHandle(typed.value) === props.handle);

async function start() {
  confirming.value = true;
  await nextTick();
  input.value?.focus();
}

function cancel() {
  confirming.value = false;
  typed.value = "";
  error.value = null;
}

async function onSubmit() {
  if (!matches.value || deleting.value) return;
  deleting.value = true;
  error.value = null;
  try {
    await deleteAccount(props.handle);
    emit("deleted");
  } catch {
    error.value = "We couldn’t delete your account. Try again.";
  } finally {
    deleting.value = false;
  }
}
</script>

<template>
  <section
    class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-rose-200 dark:bg-slate-900 dark:ring-rose-900"
    aria-labelledby="danger-zone-heading"
    data-testid="danger-zone"
  >
    <h2 id="danger-zone-heading" class="text-base font-semibold text-rose-800 dark:text-rose-300">Delete account</h2>
    <p class="text-sm text-slate-600 dark:text-slate-400">
      This permanently deletes your profile and every collection and item in it. It can’t be undone.
    </p>
    <button
      v-if="!confirming"
      type="button"
      class="inline-flex h-11 items-center rounded-md border border-rose-300 px-4 text-sm font-medium text-rose-700 hover:bg-rose-50 dark:border-rose-800 dark:text-rose-300 dark:hover:bg-rose-950"
      data-testid="delete-account-start"
      @click="start"
    >
      Delete my account…
    </button>
    <form v-else class="space-y-3" data-testid="delete-account-form" @submit.prevent="onSubmit">
      <div>
        <label :for="inputId" class="block text-sm font-medium text-slate-700 dark:text-slate-200">
          Type <span class="font-mono">@{{ handle }}</span> to confirm
        </label>
        <input
          :id="inputId"
          ref="input"
          v-model="typed"
          type="text"
          autocapitalize="none"
          autocomplete="off"
          spellcheck="false"
          class="mt-1 block h-11 w-full rounded-md border border-slate-300 bg-white px-3 text-base text-slate-900 focus:border-rose-500 focus:outline-none focus:ring-2 focus:ring-rose-300 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-50 dark:focus:ring-rose-800"
          data-testid="delete-account-confirm"
        >
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          type="submit"
          class="inline-flex h-11 items-center rounded-md bg-rose-700 px-4 text-sm font-medium text-white hover:bg-rose-800 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="!matches || deleting"
          data-testid="delete-account-submit"
        >
          {{ deleting ? "Deleting…" : "Permanently delete my account" }}
        </button>
        <button
          type="button"
          class="inline-flex h-11 items-center rounded-md border border-slate-300 px-4 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          data-testid="delete-account-cancel"
          @click="cancel"
        >
          Cancel
        </button>
      </div>
      <p v-if="error" role="alert" class="text-sm text-rose-700 dark:text-rose-300" data-testid="delete-account-error">
        {{ error }}
      </p>
    </form>
  </section>
</template>

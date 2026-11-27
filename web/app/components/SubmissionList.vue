<script setup lang="ts">
import { catalogEntryPath } from "~/utils/catalog";
import { kindLabel, statusLabel } from "~/utils/moderation";
import type { ModerationStatus, Submission } from "~/types/api";

/**
 * Your catalog suggestions and corrections with their outcomes: the
 * "notifications-lite" view (ADR 0012). Rejections show the admin's reason.
 */
defineProps<{ submissions: Submission[] }>();

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });

const badgeClass: Record<ModerationStatus, string> = {
  pending: "bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-100",
  approved: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100",
  merged: "bg-sky-100 text-sky-900 dark:bg-sky-950 dark:text-sky-100",
  rejected: "bg-rose-100 text-rose-900 dark:bg-rose-950 dark:text-rose-100",
};

/** "Name, Piece count": what a correction changes, in words. */
function changedLabels(s: Submission): string {
  const p = s.proposed;
  if (!p) return "";
  const labels: string[] = [];
  if (p.name !== undefined) labels.push("Name");
  if (p.description !== undefined) labels.push("Description");
  if (p.attributes) labels.push("Details");
  return labels.join(", ");
}
</script>

<template>
  <ul class="space-y-2" data-testid="submission-list">
    <li
      v-for="s in submissions"
      :key="s.id"
      class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
      data-testid="submission"
    >
      <div class="flex flex-wrap items-start justify-between gap-2">
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
            {{ kindLabel(s.kind) }} · {{ s.category_name }}
          </p>
          <p class="font-medium text-slate-900 dark:text-slate-50">{{ s.name }}</p>
          <p v-if="s.kind === 'catalog_correction' && changedLabels(s)" class="text-xs text-slate-600 dark:text-slate-400">
            Changes: {{ changedLabels(s) }}
          </p>
        </div>
        <span
          class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium"
          :class="badgeClass[s.status]"
          data-testid="submission-status"
        >
          {{ statusLabel(s.status, s.kind) }}
        </span>
      </div>

      <p v-if="s.status === 'merged' && s.merged_into_name" class="mt-2 text-sm text-slate-700 dark:text-slate-300">
        It’s in the catalog as “{{ s.merged_into_name }}”.
      </p>
      <p
        v-if="s.reason"
        class="mt-2 rounded-md bg-slate-50 p-2 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
        data-testid="submission-reason"
      >
        <span class="font-medium">Reviewer’s note:</span> {{ s.reason }}
      </p>

      <div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500 dark:text-slate-400">
        <span>
          Sent {{ dateFormat.format(new Date(s.created_at)) }}<template v-if="s.decided_at">
            · Reviewed {{ dateFormat.format(new Date(s.decided_at)) }}</template>
        </span>
        <NuxtLink
          v-if="s.entry_id"
          :to="catalogEntryPath(s.category_slug, s.entry_id)"
          class="font-medium text-slate-700 underline hover:no-underline dark:text-slate-200"
          data-testid="submission-entry-link"
        >
          View in the catalog
        </NuxtLink>
        <NuxtLink
          v-else-if="s.kind === 'category'"
          :to="`/categories/${s.category_slug}`"
          class="font-medium text-slate-700 underline hover:no-underline dark:text-slate-200"
          data-testid="submission-category-link"
        >
          Open the category
        </NuxtLink>
      </div>
    </li>
  </ul>
</template>

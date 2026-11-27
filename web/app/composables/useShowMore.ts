import { computed, ref, watch, type Ref } from "vue";

export interface ItemsPage<T> {
  items: T[];
  has_more: boolean;
}

/**
 * "Show more" paging on top of a first page that's fetched elsewhere.
 * Further pages are appended locally and reset whenever the first page's
 * items change (a sort/filter change or a refresh), but not when other
 * fields of the response change (e.g. a settings toggle).
 *
 * A request still in flight when the reset happens is stale: a generation
 * counter, bumped by every reset, makes its response get dropped instead of
 * appended to the new list.
 */
export function useShowMore<T>(
  firstPage: Readonly<Ref<ItemsPage<T>>>,
  fetchPage: (offset: number) => Promise<ItemsPage<T>>,
) {
  const extra = ref<T[]>([]) as Ref<T[]>;
  const extraHasMore = ref<boolean | null>(null);
  const loadingMore = ref(false);
  const error = ref(false);
  let generation = 0;

  watch(
    () => firstPage.value.items,
    () => {
      generation++;
      extra.value = [];
      extraHasMore.value = null;
      loadingMore.value = false;
      error.value = false;
    },
  );

  const items = computed(() => [...firstPage.value.items, ...extra.value]);
  const hasMore = computed(() => extraHasMore.value ?? firstPage.value.has_more);

  async function loadMore() {
    const gen = generation;
    loadingMore.value = true;
    error.value = false;
    try {
      const res = await fetchPage(items.value.length);
      if (gen !== generation) return;
      extra.value = [...extra.value, ...res.items];
      extraHasMore.value = res.has_more;
    } catch (e) {
      if (gen !== generation) return;
      console.error(e);
      error.value = true;
    } finally {
      if (gen === generation) loadingMore.value = false;
    }
  }

  return { items, hasMore, loadingMore, error, loadMore };
}

import { onScopeDispose, ref, watch, type Ref } from "vue";

/**
 * A read-only copy of `source` that only updates once `source` has stopped
 * changing for `ms` milliseconds. Search boxes use it so typing "falcon"
 * sends one request instead of six.
 */
export function useDebouncedRef<T>(source: Ref<T>, ms: number): Readonly<Ref<T>> {
  const debounced = ref(source.value) as Ref<T>;
  let timer: ReturnType<typeof setTimeout> | undefined;
  watch(source, (value) => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      debounced.value = value;
    }, ms);
  });
  onScopeDispose(() => clearTimeout(timer));
  return debounced;
}

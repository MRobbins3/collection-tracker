import { describe, it, expect, vi, afterEach } from "vitest";
import { effectScope, nextTick, ref } from "vue";
import { collapseWhitespace } from "~/utils/text";
import { useDebouncedRef } from "~/composables/useDebouncedRef";

describe("collapseWhitespace as a search-box normalizer", () => {
  it.each([
    ["  star   wars ", "star wars"],
    ["\tmillennium\n falcon", "millennium falcon"],
    ["   ", ""],
    ["", ""],
    [null, ""],
    [undefined, ""],
    ["UCS", "UCS"],
  ])("%j → %j", (input, expected) => {
    expect(collapseWhitespace(input)).toBe(expected);
  });
});

describe("useDebouncedRef", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("updates only after the source stops changing for the delay", async () => {
    vi.useFakeTimers();
    const scope = effectScope();
    const source = ref("");
    const debounced = scope.run(() => useDebouncedRef(source, 250))!;

    source.value = "f";
    await nextTick();
    vi.advanceTimersByTime(200);
    source.value = "fa";
    await nextTick();
    vi.advanceTimersByTime(200);
    expect(debounced.value).toBe("");

    vi.advanceTimersByTime(50);
    expect(debounced.value).toBe("fa");
    scope.stop();
  });

  it("starts with the source's current value", () => {
    const scope = effectScope();
    const debounced = scope.run(() => useDebouncedRef(ref("lego"), 250))!;
    expect(debounced.value).toBe("lego");
    scope.stop();
  });

  it("drops a pending update when its scope is disposed", async () => {
    vi.useFakeTimers();
    const scope = effectScope();
    const source = ref("");
    const debounced = scope.run(() => useDebouncedRef(source, 250))!;
    source.value = "x";
    await nextTick();
    scope.stop();
    vi.advanceTimersByTime(500);
    expect(debounced.value).toBe("");
  });
});

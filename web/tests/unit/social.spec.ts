import { describe, it, expect } from "vitest";
import { requestCountLabel, socialErrorMessage } from "~/utils/social";

describe("socialErrorMessage", () => {
  it("prefers the API's own message", () => {
    expect(socialErrorMessage({ data: { error: "@noah isn’t accepting friend requests." } })).toBe(
      "@noah isn’t accepting friend requests.",
    );
  });
  it("explains a 404 without blaming anyone", () => {
    expect(socialErrorMessage({ statusCode: 404 })).toBe("That person or request isn’t available any more.");
  });
  it("falls back to a generic message", () => {
    expect(socialErrorMessage(new Error("boom"))).toBe("Something went wrong. Please try again.");
  });
});

describe("requestCountLabel", () => {
  it("pluralizes", () => {
    expect(requestCountLabel(1)).toBe("1 request waiting");
    expect(requestCountLabel(3)).toBe("3 requests waiting");
  });
});

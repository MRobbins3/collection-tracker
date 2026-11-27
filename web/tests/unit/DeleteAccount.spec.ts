import { describe, it, expect, vi, beforeEach } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import DeleteAccount from "~/components/DeleteAccount.vue";

let deleteAccount: ReturnType<typeof vi.fn>;

beforeEach(() => {
  deleteAccount = vi.fn(async () => {});
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useProfileActions = () => ({ update: vi.fn(), checkHandle: vi.fn(), deleteAccount });
});

async function openConfirm() {
  const wrapper = mount(DeleteAccount, { props: { handle: "emma" } });
  await wrapper.get('[data-testid="delete-account-start"]').trigger("click");
  return wrapper;
}

describe("DeleteAccount", () => {
  it("asks for the handle before anything can be deleted", async () => {
    const wrapper = await openConfirm();
    const input = wrapper.get('[data-testid="delete-account-confirm"]');
    expect(wrapper.get(`label[for="${input.attributes("id")}"]`).text()).toContain("@emma");
    expect(wrapper.get('[data-testid="delete-account-submit"]').attributes("disabled")).toBeDefined();

    await input.setValue("emm");
    expect(wrapper.get('[data-testid="delete-account-submit"]').attributes("disabled")).toBeDefined();
  });

  it("deletes once the handle matches (case and @ don't matter)", async () => {
    const wrapper = await openConfirm();
    await wrapper.get('[data-testid="delete-account-confirm"]').setValue(" @Emma ");
    expect(wrapper.get('[data-testid="delete-account-submit"]').attributes("disabled")).toBeUndefined();
    await wrapper.get('[data-testid="delete-account-form"]').trigger("submit");
    await flushPromises();
    expect(deleteAccount).toHaveBeenCalledWith("emma");
    expect(wrapper.emitted("deleted")).toHaveLength(1);
  });

  it("can be cancelled", async () => {
    const wrapper = await openConfirm();
    await wrapper.get('[data-testid="delete-account-cancel"]').trigger("click");
    expect(wrapper.find('[data-testid="delete-account-confirm"]').exists()).toBe(false);
    expect(deleteAccount).not.toHaveBeenCalled();
  });

  it("says so when deletion fails", async () => {
    deleteAccount.mockRejectedValueOnce(new Error("boom"));
    const wrapper = await openConfirm();
    await wrapper.get('[data-testid="delete-account-confirm"]').setValue("emma");
    await wrapper.get('[data-testid="delete-account-form"]').trigger("submit");
    await flushPromises();
    expect(wrapper.get('[data-testid="delete-account-error"]').text()).toContain("couldn’t delete");
    expect(wrapper.emitted("deleted")).toBeUndefined();
  });
});

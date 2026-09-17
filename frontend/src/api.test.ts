import { afterEach, describe, expect, it, vi } from "vitest";
import { api, post, APIError } from "./api";
afterEach(() => vi.unstubAllGlobals());
describe("API client", () => {
  it("preserves the backend error and status", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(
            JSON.stringify({ error: { message: "session expired" } }),
            { status: 401 },
          ),
        ),
    );
    await expect(api("/api/events")).rejects.toEqual(
      new APIError(401, "session expired"),
    );
  });
  it("posts JSON and includes same-origin cookies", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ id: "prj_test" }), { status: 201 }),
      );
    vi.stubGlobal("fetch", fetcher);
    await post("/api/projects", { name: "Orders" });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/projects",
      expect.objectContaining({
        credentials: "same-origin",
        method: "POST",
        body: '{"name":"Orders"}',
      }),
    );
  });
  it("handles an empty delete response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(null, { status: 204 })),
    );
    expect(await api("/api/api-keys/id", { method: "DELETE" })).toBeUndefined();
  });
});

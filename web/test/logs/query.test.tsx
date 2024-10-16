import { renderHook, waitFor } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
// import { http, HttpResponse } from "msw";
import { type ParentProps, createSignal } from "solid-js";
import { describe, expect, it } from "vitest";
import { createLoqQuery } from "../../src/lib/store/query";

// const handlers = [
//     http.get(`${import.meta.env.VITE_API_URL}/v1/logs/*`, () => {
//         return HttpResponse.json({
//             data: {
//                 logs: [
//                     { id: 1, message: "Hello" },
//                     { id: 2, message: "World" },
//                 ],
//             },
//         });
//     }),
// ];

describe("createLoqQuery", () => {
    const queryClient = new QueryClient();

    const Wrapper = (props: ParentProps) => (
        <QueryClientProvider client={queryClient}>
            {props.children}
        </QueryClientProvider>
    );

    const [clientId, _setClientId] = createSignal("marko");
    const [selectedInstances, _setSelectedInstances] = createSignal<
        string[] | undefined
    >(undefined);
    const [search, _setSearch] = createSignal<string | undefined>(undefined);

    it("should initialize and return data", async () => {
        const { result } = renderHook(
            () => createLoqQuery(clientId, selectedInstances, search),
            {
                wrapper: Wrapper,
            },
        );

        await waitFor(() => result.queryDetails.isSuccess);
        expect(result.queryDetails.data).toEqual({});
    });
});

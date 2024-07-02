import { type RouteDefinition, useParams } from "@solidjs/router";
import {
    type FetchInfiniteQueryOptions,
    createInfiniteQuery,
    useQueryClient,
} from "@tanstack/solid-query";
import { createVirtualizer } from "@tanstack/solid-virtual";
import { For, createEffect, onCleanup, onMount } from "solid-js";

type LogLine = {
    id: string;
    timestamp: number;
    content: string;
};

const getLogsQueryOptions = (queryClient: string) =>
    ({
        queryKey: ["logs", queryClient],
        queryFn: async (params) => {
            console.log("Received params", params.pageParam);
            const response = await fetch(params.pageParam as string);
            return response.json() as Promise<LogLine[]>;
        },
        initialPageParam: buildUrl(queryClient),
    }) satisfies FetchInfiniteQueryOptions;

const buildUrl = (
    clieentId: string,
    params?: {
        cursorId: string;
        cursorTime: number;
        direction: "forward" | "backward";
    },
) => {
    let url = `http://localhost:1323/api/v1/logs/${clieentId}`;
    if (params) {
        url += `?cursorId=${params.cursorId}&cursorTime=${params.cursorTime}&direction=${params.direction}`;
    }
    return url;
};

const getLogsPage = (queryClient: string) =>
    createInfiniteQuery(() => ({
        ...getLogsQueryOptions(queryClient),
        getNextPageParam: (lastPage) => {
            const last = lastPage[lastPage.length - 1];
            if (!last) {
                return null;
            }
            return buildUrl(queryClient, {
                cursorId: last.id,
                cursorTime: last.timestamp,
                direction: "forward",
            });
        },
        getPreviousPageParam: (lastPage) => {
            const first = lastPage[0];
            if (!first) {
                return null;
            }
            return buildUrl(queryClient, {
                cursorId: first.id,
                cursorTime: first.timestamp,
                direction: "backward",
            });
        },
        staleTime: 1000,
    }));

export const route = {
    load: ({ params }) => {
        const clientId = params.clientId;
        console.log("Loading logs for client", clientId);
        const queryClient = useQueryClient();
        return queryClient.fetchInfiniteQuery(getLogsQueryOptions(clientId));
    },
} satisfies RouteDefinition;

export default () => {
    // biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
    let logsRef: HTMLDivElement | undefined = undefined;
    // biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
    let topRef: HTMLDivElement | undefined = undefined;
    // biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
    let bottomRef: HTMLDivElement | undefined = undefined;

    const clientId = useParams<{ clientId: string }>().clientId;
    const logs = getLogsPage(clientId);
    const logsOrEmpty = () => logs.data?.pages.flat() ?? [];
    const logCount = () => logsOrEmpty().length;

    const virtualizer = createVirtualizer({
        get count() {
            return logCount();
        },
        estimateSize: () => 25,
        getScrollElement: () => logsRef ?? null,
    });

    createEffect(() => {
        if (logs.data?.pages.length === 1) {
            if (logsRef) {
                // Scroll to bottom
                (logsRef as HTMLDivElement).scrollTop = (
                    logsRef as HTMLDivElement
                ).scrollHeight;
            }
        }
    });

    const observer = new IntersectionObserver((entries) => {
        for (const entry of entries) {
            if (entry.isIntersecting) {
                console.log("Intersecting", logs.hasNextPage, logs.hasPreviousPage);
                if (
                    entry.target.id === "top" &&
                    logs.hasNextPage &&
                    !logs.isFetchingNextPage &&
                    !logs.isLoading &&
                    logs.hasNextPage
                ) {
                    console.log("Fetching next page");
                    //logs.fetchNextPage()
                } else if (
                    entry.target.id === "bottom" &&
                    logs.hasPreviousPage &&
                    !logs.isFetchingPreviousPage &&
                    !logs.isLoading &&
                    logs.hasPreviousPage
                ) {
                    logs.fetchPreviousPage();
                    console.log("Fetching previous page");
                }
            }
        }
    });

    onMount(() => {
        if (topRef) {
            observer.observe(topRef);
        }
        if (bottomRef) {
            observer.observe(bottomRef);
        }
    });

    onCleanup(() => {
        observer.disconnect();
    });

    return (
        <>
            <div
                ref={logsRef}
                style={{
                    height: "90vh",
                    width: "100%",
                    overflow: "auto",
                }}
            >
                <div
                    style={{
                        height: `${virtualizer.getTotalSize()}px`,
                        width: "100%",
                        position: "relative",
                    }}
                >
                    <div id="top" ref={topRef} />
                    <For each={virtualizer.getVirtualItems()}>
                        {(virtualItem) => {
                            return (
                                <div
                                    style={{
                                        position: "absolute",
                                        top: 0,
                                        left: 0,
                                        width: "100%",
                                        height: `${virtualItem.size}px`,
                                        transform: `translateY(${virtualItem.start}px)`,
                                    }}
                                >
                                    <pre class="text-white">
                                        {logsOrEmpty()[virtualItem.index].content}
                                    </pre>
                                </div>
                            );
                        }}
                    </For>
                    <div id="bottom" ref={bottomRef} />
                </div>
            </div>
        </>
    );
};

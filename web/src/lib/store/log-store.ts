import {
    type QueryClient,
    useQueryClient,
    createInfiniteQuery,
} from "@tanstack/solid-query";
import { createStore } from "solid-js/store";
import { createEffect, createSignal, on } from "solid-js";
import {
    type SortFn,
    SortedList,
    treeNodeToCursor,
} from "~/lib/store/sorted-list";

export type Client = {
    clientId: string;
    ipAddress: string;
};
export type LogLine = {
    id: string;
    timestamp: number;
    sequenceNumber: number;
    content: string;
    client: Client;
};

export type LogPageCursor = {
    cursorSequenceNumber: number;
    cursorTime: number;
    direction: "forward" | "backward";
};

export type CreateLogQueryResult = ReturnType<typeof newCreateLogQuery>;

export const getInstances = async (
    clientId: string,
    selectedInstances: string[],
    abortSignal?: AbortSignal,
) => {
    const searchParams = new URLSearchParams();
    for (const instance of selectedInstances) {
        searchParams.append("instances", instance);
    }

    return fetch(
        `${import.meta.env.VITE_API_URL}/v1/logs/${clientId}/instances?${searchParams.toString()}`,
        {
            signal: abortSignal,
        },
    ).then(async (res) => {
        if (!res.ok) {
            throw Error(await res.text());
        }

        return res.json() as Promise<string[]>;
    });
};

export const createLogQuery = (
    params: () => {
        clientId: string;
        selectedInstances: string[];
        search?: string;
    },
) => {
    const clientId = () => params().clientId;
    const search = () => params().search;

    const queryClient = useQueryClient();
    const [state, setState] = createStore(
        createDefaultState(
            queryClient,
            clientId(),
            params().selectedInstances,
            search(),
        ),
    );

    createEffect(
        on(params, (latestParams) => {
            setState(
                createDefaultState(
                    queryClient,
                    latestParams.clientId,
                    latestParams.selectedInstances,
                    latestParams.search,
                ),
            );

            dispatchEvent(new Event("scroll-to-bottom"));
        }),
    );

    const fetchNextPage = async () => {
        console.warn("Not implemented");
    };

    const fetchPreviousPage = async () => {
        if (!state.logStore) {
            throw new Error("Log store not initialized");
        }
        setState("isPreviousPageLoading", true);

        const lastLog = state.logStore.getHead();
        try {
            const logs = await queryClient.fetchQuery({
                queryKey: ["logs", clientId, search, lastLog?.value],
                queryFn: async () => {
                    return fetchLogs(
                        clientId(),
                        search(),
                        params().selectedInstances,
                        treeNodeToCursor(lastLog),
                    );
                },
            });
            setState("lastLoadedPageSize", logs.length);
            state.logStore.insertMany(logs);
        } catch (_error) {
            setState("isPreviousPageError", true);
        }
        setState("isPreviousPageLoading", false);
    };

    return {
        fetchPreviousPage,
        fetchNextPage,
        isNextPageLoading: state.isNextPageLoading,
        isNextPageError: state.isNextPageError,
        isPreviousPageLoading: state.isPreviousPageLoading,
        isPreviousPageError: state.isPreviousPageError,
        state,
    };
};

export const newCreateLogQuery = (
    params: () => {
        clientId: string;
        selectedInstances: string[];
        search?: string;
    },
) => {
    const [logStore, setLogStore] = createSignal(new SortedList<LogLine>(sortFn));
    const [lastLoadedPageSize, setLastLoadedPageSize] = createSignal(0);
    const [logCount, setLogCount] = createSignal(0);

    createEffect(
        on(params, () => {
            setLogStore(new SortedList<LogLine>(sortFn));
        }),
    );

    createEffect(on(logStore, () => {
        setLogCount(logStore().size);
    }))

	const query = createInfiniteQuery(() => ({
        queryKey: [
            "logs",
            params().clientId,
            params().search,
            params().selectedInstances,
        ] as const,
        queryFn: async ({ pageParam, queryKey }) => {
            const logs = await fetchLogs(queryKey[1], queryKey[2], queryKey[3], pageParam);
            logStore().insertMany(logs);
            setLogCount(logStore().size);
            console.log("Fetched logs", logStore().size);
            setLastLoadedPageSize(logs.length);
            return logs;
        },
        initialPageParam: null,
        getPreviousPageParam: () => {
            const lastLog = logStore().getHead();
            return treeNodeToCursor(lastLog);
        },
        getNextPageParam: () => {
            return undefined;
        },
    }));

    return {
        query,
        logStore,
        logCount,
        lastLoadedPageSize,
    };
};

export const fetchLogs = async (
    clientId: string,
    search?: string,
    selectedInstances?: string[],
    cursor?: LogPageCursor | null,
) => {
    const response = await fetch(
        buildUrl(clientId, search, selectedInstances, cursor),
    );
    return response.json() as Promise<LogLine[]>;
};

const buildBaseUrl = (clientId: string) =>
    `${import.meta.env.VITE_API_URL}/v1/logs/${clientId}`;

const buildUrl = (
    clientId: string,
    search?: string,
    selectedInstances?: string[],
    params?: LogPageCursor | null,
) => {
    let url = buildBaseUrl(clientId);
    const searchParams = new URLSearchParams();
    if (params) {
        searchParams.append(
            "cursorSequenceNumber",
            `${params.cursorSequenceNumber}`,
        );
        searchParams.append("cursorTime", `${params.cursorTime}`);
        searchParams.append("direction", params.direction);
    }
    if (selectedInstances) {
        for (const instance of selectedInstances) {
            searchParams.append("instance", instance);
        }
    }
    if (search) {
        searchParams.append("search", search);
        url += "/search";
    }
    if (searchParams.toString()) {
        url += `?${searchParams.toString()}`;
    }
    return url;
};

const createDefaultState = (
    queryClient: QueryClient,
    clientId: string,
    selectedInstances: string[],
    search?: string,
) => {
    const defaultQueryState = {
        isPreviousPageLoading: false,
        isPreviousPageError: false,
        isNextPageLoading: false,
        isNextPageError: false,
        lastLoadedPageSize: 0,
    };
    let logStore = queryClient.getQueryData<SortedList<LogLine>>([
        "logs",
        clientId,
        selectedInstances,
        search,
    ]);

    if (!logStore) {
        logStore = new SortedList<LogLine>(sortFn);
        queryClient.setQueryData<SortedList<LogLine>>(
            ["logs", clientId, selectedInstances, search],
            logStore,
        );
    }

    return {
        ...defaultQueryState,
        logStore,
    };
};

const sortFn: SortFn<LogLine> = (a, b) => {
    const timestampDiff = a.timestamp - b.timestamp;
    if (timestampDiff !== 0) {
        return timestampDiff;
    }
    return a.sequenceNumber - b.sequenceNumber;
};

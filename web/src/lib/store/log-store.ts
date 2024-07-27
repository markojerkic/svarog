import { createReconnectingWS } from "@solid-primitives/websocket";
import { useQueryClient } from "@tanstack/solid-query";
import { createSignal, onCleanup, onMount } from "solid-js";
import { SortedList, treeNodeToCursor } from "~/lib/store/sorted-list";

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

export type CreateLogQueryResult = ReturnType<typeof createLogQuery>;

export const createLogSubscription = (
	clientId: string,
	logStore: SortedList<LogLine>,
) => {
	const ws = createReconnectingWS(`ws://localhost:1323/api/v1/ws/${clientId}`);

	onMount(() => {
		ws.addEventListener("message", (e) => {
			const message = e.data;
			console.log("Message", message);
		});
	});

	onCleanup(() => {
		ws.close();
	});
};

export const createLogQuery = (clientId: string, search?: string) => {
	const queryClient = useQueryClient();
	const [isPreviousPageLoading, setIsPreviousPageLoading] = createSignal(false);
	const [isPreviousPageError, setIsPreviousPageError] = createSignal(false);
	const [isNextPageLoading] = createSignal(false);
	const [isNextPageError] = createSignal(false);
	const [lastLoadedPageSize, setLastLoadedPageSize] = createSignal(0);

	let logStore = queryClient.getQueryData<SortedList<LogLine>>([
		"logs",
		clientId,
		search,
	]);

	if (!logStore) {
		logStore = new SortedList<LogLine>((a, b) => {
			const timestampDiff = a.timestamp - b.timestamp;
			if (timestampDiff !== 0) {
				return timestampDiff;
			}
			return a.sequenceNumber - b.sequenceNumber;
		});
		queryClient.setQueryData<SortedList<LogLine>>(
			["logs", clientId, search],
			logStore,
		);
	}

	const fetchNextPage = async () => {
		console.warn("Not implemented");
	};

	const fetchPreviousPage = async () => {
		if (!logStore) {
			throw new Error("Log store not initialized");
		}
		setIsPreviousPageLoading(true);

		const lastLog = logStore.getHead();
		try {
			const logs = await queryClient.fetchQuery({
				queryKey: ["logs", clientId, search, lastLog?.value],
				queryFn: async () => {
					return fetchLogs(clientId, search, treeNodeToCursor(lastLog));
				},
			});
			setLastLoadedPageSize(logs.length);
			logStore.insertMany(logs);
		} catch (error) {
			setIsPreviousPageError(true);
		}
		setIsPreviousPageLoading(false);
	};

	return {
		logStore,
		lastLoadedPageSize,
		fetchPreviousPage,
		fetchNextPage,
		isPreviousPageLoading,
		isPreviousPageError,
		isNextPageLoading,
		isNextPageError,
	};
};

export const fetchLogs = async (
	clientId: string,
	search?: string,
	cursor?: LogPageCursor | null,
) => {
	const response = await fetch(buildUrl(clientId, search, cursor));
	return response.json() as Promise<LogLine[]>;
};

const buildBaseUrl = (clientId: string) =>
	`http://localhost:1323/api/v1/logs/${clientId}`;

const buildUrl = (
	clientId: string,
	search?: string,
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
	if (search) {
		searchParams.append("search", search);
		url += "/search";
	}
	if (searchParams.toString()) {
		url += `?${searchParams.toString()}`;
	}
	return url;
};

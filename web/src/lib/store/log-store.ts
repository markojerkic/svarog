import { type QueryClient, useQueryClient } from "@tanstack/solid-query";
import { createStore } from "solid-js/store";
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

export type CreateLogQueryResult = ReturnType<typeof createLogQuery>;

const createDefaultState = (
	queryClient: QueryClient,
	clientId: string,
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
		search,
	]);

	if (!logStore) {
		logStore = new SortedList<LogLine>(sortFn);
		queryClient.setQueryData<SortedList<LogLine>>(
			["logs", clientId, search],
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

export const createLogQuery = (clientId: string, search?: string) => {
	const queryClient = useQueryClient();
	const [state, setState] = createStore(
		createDefaultState(queryClient, clientId, search),
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
					return fetchLogs(clientId, search, treeNodeToCursor(lastLog));
				},
			});
			setState("lastLoadedPageSize", logs.length);
			state.logStore.insertMany(logs);
		} catch (error) {
			setState("isPreviousPageError", true);
		}
		setState("isPreviousPageLoading", false);
	};

	return {
		fetchPreviousPage,
		fetchNextPage,
		state,
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
	`${import.meta.env.VITE_API_URL}/v1/logs/${clientId}`;

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

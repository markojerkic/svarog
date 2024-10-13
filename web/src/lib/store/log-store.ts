import { type QueryClient, useQueryClient } from "@tanstack/solid-query";
import { createStore } from "solid-js/store";
import { createEffect, on } from "solid-js";
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

export const getInstances = async (
	clientId: string,
	abortSignal?: AbortSignal,
) => {
	return fetch(
		`${import.meta.env.VITE_API_URL}/v1/logs/${clientId}/instances`,
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
				queryKey: [
					"logs",
					"log-page",
					clientId(),
					params().selectedInstances,
					search(),
					lastLog,
				] as const,
				queryFn: async ({ queryKey }) => {
					return fetchLogs(
						queryKey[2],
						queryKey[3],
						queryKey[4],
						treeNodeToCursor(queryKey[5]),
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

export const fetchLogs = async (
	clientId: string,
	selectedInstances?: string[],
	search?: string,
	cursor?: LogPageCursor | null,
) => {
	const response = await fetch(
		buildUrl(clientId, selectedInstances, search, cursor),
	);
	return response.json() as Promise<LogLine[]>;
};

const buildBaseUrl = (clientId: string) =>
	`${import.meta.env.VITE_API_URL}/v1/logs/${clientId}`;

const buildUrl = (
	clientId: string,
	selectedInstances?: string[],
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

	if (selectedInstances) {
		for (const instance of selectedInstances) {
			searchParams.append("instances", instance);
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
	console.log("Creating new default state");

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

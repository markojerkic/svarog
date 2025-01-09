import { createInfiniteQuery, createQuery } from "@tanstack/solid-query";
import { createEffect, createSignal, type Accessor } from "solid-js";
import { type SortFn, SortedList } from "@/lib/store/sorted-list";
import { api } from "../utils/axios-api";

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
	clientId: Accessor<string>,
	selectedInstances: Accessor<string[] | undefined>,
	searchQuery: Accessor<string | undefined>,
) => {
	const [logs, setLogs] = createSignal<LogLine[]>([]);
	const query = createInfiniteQuery(() => ({
		queryKey: ["logs", clientId(), selectedInstances(), searchQuery()],
		queryFn: async ({ pageParam, signal }) => {
			return fetchLogPage(
				clientId(),
				{
					selectedInstances: selectedInstances(),
					search: searchQuery(),
					cursor: pageParam,
				},
				signal,
			);
		},
		initialPageParam: undefined as LogPageCursor | undefined,
		getNextPageParam: () => undefined,
		getPreviousPageParam: (firstPage) => {
			return {
				direction: "backward",
				cursorTime: firstPage[0].timestamp,
				cursorSequenceNumber: firstPage[0].sequenceNumber,
			} satisfies LogPageCursor;
		},
	}));

	createEffect(() => {
		setLogs(query.data?.pages.flat() ?? []);
	});

	return {
		get logs() {
			return logs();
		},
		query,
	};
};

export const logsSortFn: SortFn<LogLine> = (a, b) => {
	const timestampDiff = a.timestamp - b.timestamp;
	if (timestampDiff !== 0) {
		return timestampDiff;
	}
	return a.sequenceNumber - b.sequenceNumber;
};

const fetchLogPage = async (
	clientId: string,
	options: FetchLogPageOptions,
	abortSignal: AbortSignal,
) => {
	const response = await api.get<LogLine[]>(`/v1/logs/${clientId}`, {
		params: options,
		signal: abortSignal,
	});

	return response.data;
};

type FetchLogPageOptions = {
	selectedInstances?: string[];
	search?: string;
	cursor?: LogPageCursor | null;
};

export type CreateLogQueryResult = ReturnType<typeof createLogQuery>;
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

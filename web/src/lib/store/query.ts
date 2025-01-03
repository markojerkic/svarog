import { createInfiniteQuery, createQuery } from "@tanstack/solid-query";
import { createSignal, type Accessor } from "solid-js";
import { type SortFn, SortedList } from "@/lib/store/sorted-list";

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
	const store = createQuery(() => ({
		queryKey: ["store", "logs", clientId(), selectedInstances(), searchQuery()],
		initialData: new SortedList<LogLine>(logsSortFn),
		staleTime: Number.POSITIVE_INFINITY,
		queryFn: () => {
			console.log("Resetting data");
			return new SortedList<LogLine>(logsSortFn);
		},
	}));
	const [lastLoadedPageSize, setLastLoadedPageSize] = createSignal(0);

	const query = createInfiniteQuery(() => ({
		queryKey: ["logs", clientId(), selectedInstances(), searchQuery()] as const,
		initialPageParam: undefined as LogPageCursor | undefined,
		gcTime: 1000 * 30,
		queryFn: async ({ queryKey, pageParam, signal }) => {
			const page = await fetchLogPage(
				queryKey[1],
				{
					selectedInstances: queryKey[2],
					search: queryKey[3],
					cursor: pageParam,
				},
				signal,
			);

			store.data.insertMany(page);
			setLastLoadedPageSize(page.length);

			return page;
		},
		getNextPageParam: () => {
			return undefined;
			// return {
			// 	direction: "forward",
			// 	cursorTime: lastPage[lastPage.length - 1].timestamp,
			// 	cursorSequenceNumber: lastPage[lastPage.length - 1].sequenceNumber,
			// } satisfies LogPageCursor | undefined;
		},
		getPreviousPageParam: (firstPage) => {
			return {
				direction: "backward",
				cursorTime: firstPage[0].timestamp,
				cursorSequenceNumber: firstPage[0].sequenceNumber,
			} satisfies LogPageCursor | undefined;
		},
	}));

	return {
		get data() {
			return store.data;
		},
		get logCount() {
			return store.data.size;
		},
		lastLoadedPageSize,
		queryDetails: query,
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
	const response = await fetch(buildUrl(clientId, options), {
		signal: abortSignal,
	});
	return response.json() as Promise<LogLine[]>;
};

const buildUrl = (clientId: string, options: FetchLogPageOptions) => {
	let url = buildBaseUrl(clientId);
	const searchParams = new URLSearchParams();
	if (options.cursor) {
		searchParams.append(
			"cursorSequenceNumber",
			`${options.cursor.cursorSequenceNumber}`,
		);
		searchParams.append("cursorTime", `${options.cursor.cursorTime}`);
		searchParams.append("direction", options.cursor.direction);
	}

	if (options.selectedInstances) {
		for (const instance of options.selectedInstances) {
			searchParams.append("instances", instance);
		}
	}

	if (options.search) {
		searchParams.append("search", options.search);
		url += "/search";
	}

	if (searchParams.toString()) {
		url += `?${searchParams.toString()}`;
	}
	return url;
};

const buildBaseUrl = (clientId: string) =>
	`${import.meta.env.VITE_API_URL}/v1/logs/${clientId}`;

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

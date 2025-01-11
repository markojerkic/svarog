import type {
	InfiniteData,
	UndefinedInitialDataInfiniteOptions,
} from "@tanstack/solid-query";
import type { SortFn } from "@/lib/store/sorted-list";
import { api } from "../utils/axios-api";

type FetchLogPageOptions = {
	selectedInstances?: string[];
	search?: string;
	cursor?: LogPageCursor | null;
};

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

export const createLogQueryOptions = (
	props: () => {
		clientId: string;
		selectedInstances: string[] | undefined;
		searchQuery?: string | undefined;
	},
) => {
	return {
		queryKey: [
			"logs",
			props().clientId,
			props().selectedInstances,
			props().searchQuery,
		],
		queryFn: async ({ pageParam, signal }) => {
			return fetchLogPage(
				props().clientId,
				{
					selectedInstances: props().selectedInstances,
					search: props().searchQuery,
					cursor: pageParam as LogPageCursor,
				},
				signal,
			);
		},
		initialPageParam: undefined as LogPageCursor | undefined,
		getNextPageParam: () => undefined,
		getPreviousPageParam: (firstPage) => {
			if (firstPage.length === 0) {
				return undefined;
			}

			return {
				direction: "backward",
				cursorTime: firstPage[0].timestamp,
				cursorSequenceNumber: firstPage[0].sequenceNumber,
			} satisfies LogPageCursor;
		},
	} satisfies ReturnType<UndefinedInitialDataInfiniteOptions<LogLine[]>>;
};

export const insertLogLine = (
	store: InfiniteData<LogLine[], LogPageCursor | undefined>,
	logLine: LogLine,
) => {
	// Find position using binary search and _logsSortFn
	// Insert logLine at the correct position
	const pages = store.pages;
	let targetPageIndex = -1;
	let insertIndex = -1;

	// Find the correct page using binary search
	let left = 0;
	let right = pages.length - 1;

	while (left <= right) {
		const mid = Math.floor((left + right) / 2);
		const page = pages[mid];

		if (page.length === 0) {
			right = mid - 1;
			continue;
		}

		const firstLine = page[0];
		const lastLine = page[page.length - 1];

		if (logsSortFn(logLine, firstLine) < 0) {
			right = mid - 1;
		} else if (logsSortFn(logLine, lastLine) > 0) {
			left = mid + 1;
		} else {
			targetPageIndex = mid;
			break;
		}
	}

	// If no suitable page found, insert at the boundary
	if (targetPageIndex === -1) {
		targetPageIndex = left;
	}

	// Handle case where it should go at the end
	if (targetPageIndex >= pages.length) {
		targetPageIndex = pages.length - 1;
	}

	// Find position within the target page
	const targetPage = pages[targetPageIndex];
	left = 0;
	right = targetPage.length - 1;

	while (left <= right) {
		const mid = Math.floor((left + right) / 2);
		const comparison = logsSortFn(logLine, targetPage[mid]);

		if (comparison < 0) {
			right = mid - 1;
		} else {
			left = mid + 1;
		}
	}

	insertIndex = left;

	// Create new store with inserted log line
	return {
		...store,
		pages: pages.map((page, index) =>
			index === targetPageIndex
				? [...page.slice(0, insertIndex), logLine, ...page.slice(insertIndex)]
				: page,
		),
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
	let url = `/v1/logs/${clientId}`;
	if (options.search) {
		url += "/search";
	}
	const response = await api.get<LogLine[]>(url, {
		params: {
			...options,
			...buildCursor(options.cursor),
			cursor: undefined,
		},
		signal: abortSignal,
	});

	return response.data;
};

const buildCursor = (cursor: LogPageCursor | null | undefined) => {
	return cursor
		? {
				...cursor,
				cursorTime: `${cursor.cursorTime}`,
			}
		: {};
};

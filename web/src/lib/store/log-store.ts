import { useQueryClient } from "@tanstack/solid-query";
import { createStore } from "solid-js/store";
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

export const createLogQuery = (clientId: string) => {
	const queryClient = useQueryClient();
	const [states, setStates] = createStore({
		isPreviousPageLoading: false,
		isPreviousPageError: false,
		isNextPageLoading: false,
		isNextPageError: false,
	});

	let logStore = queryClient.getQueryData<SortedList<LogLine>>([
		"logs",
		clientId,
	]);

	if (!logStore) {
		logStore = new SortedList<LogLine>((a, b) => a.timestamp - b.timestamp);
		queryClient.setQueryData<SortedList<LogLine>>(["logs", clientId], logStore);
	}

    const fetchNextPage = async () => {
        console.warn("Not implemented");
    }

	const fetchPreviousPage = async () => {
		if (!logStore) {
			throw new Error("Log store not initialized");
		}
		setStates("isPreviousPageLoading", true);

		const lastLog = logStore.getTail();
		try {
			const logs = await queryClient.fetchQuery({
				queryKey: ["logs", clientId, lastLog?.value],
				queryFn: async () => {
					return fetchLogs(clientId, treeNodeToCursor(lastLog));
				},
			});
			logStore.insertMany(logs);
		} catch (error) {
			setStates("isPreviousPageError", true);
		}
		setStates("isPreviousPageLoading", false);
	};

	return {
		logStore,
		fetchPreviousPage,
        fetchNextPage,
		isPreviousPageLoading: states.isPreviousPageLoading,
		isPreviousPageError: states.isPreviousPageError,
		isNextPageLoading: states.isNextPageLoading,
		isNextPageError: states.isNextPageError,
	};
};

const fetchLogs = async (clientId: string, cursor: LogPageCursor | null) => {
	const response = await fetch(buildUrl(clientId, cursor));
	return response.json() as Promise<LogLine[]>;
};

const buildBaseUrl = (clientId: string) =>
	`http://localhost:1323/api/v1/logs/${clientId}`;

const buildUrl = (clientId: string, params?: LogPageCursor | null) => {
	let url = buildBaseUrl(clientId);
	if (params) {
		url += `?cursorSequenceNumber=${params.cursorSequenceNumber}&cursorTime=${params.cursorTime}&direction=${params.direction}`;
	}
	return url;
};

import { createMachine } from "@solid-primitives/state-machine";
import { SortedList } from "../store/sorted-list";
import { logsSortFn } from "../store/query";
import { useQueryClient } from "@tanstack/solid-query";
import { createEffect, createSignal, on } from "solid-js";
import { api } from "../utils/axios-api";
import { useScrollEvent } from "./use-scroll-event";

type LogStoreProps = () => {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
};
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

type RestStateMachine = {
	reset: () => void;
};

export const useLogStore = (props: LogStoreProps) => {
	const queryClient = useQueryClient();
	const [logStore, setLogStore] = createSignal(
		new SortedList<LogLine>(logsSortFn),
	);
	const scrollEventBus = useScrollEvent();

	const fetchPage = (cursor: LogPageCursor | null) => {
		return queryClient.fetchQuery({
			queryKey: [
				"logs",
				props().clientId,
				props().selectedInstances,
				props().searchQuery,
				cursor,
			],
			queryFn: async ({ signal }) => {
				return fetchLogPage(
					props().clientId,
					{
						selectedInstances: props().selectedInstances,
						search: props().searchQuery,
						cursor: cursor,
					},
					signal,
				);
			},
		});
	};

	const machine = createMachine<{
		initial: {
			to: "idle";
			value: RestStateMachine;
		};
		idle: {
			to: "initial" | "fetchingPreviousPage" | "fetchingNextPage";
			value: RestStateMachine & {
				fetchPreviousPage: () => void;
				fetchNextPage: () => void;
			};
		};
		fetchingNextPage: {
			to: "idle" | "initial";
			value: RestStateMachine;
		};
		fetchingPreviousPage: {
			to: "idle" | "initial";
			value: RestStateMachine;
		};
	}>({
		initial: "idle",
		states: {
			initial(_, to) {
				setLogStore(new SortedList<LogLine>(logsSortFn));
				fetchPage(null).then((page) => {
					logStore().insertMany(page);
					to("idle");
					scrollEventBus.scrollToIndex(page.length);
				});

				return {
					reset: () => {
						setLogStore(new SortedList<LogLine>(logsSortFn));
					},
				};
			},

			idle(_, to) {
				return {
					reset: () => {
						to("initial");
					},
					fetchPreviousPage: () => to("fetchingPreviousPage"),
					fetchNextPage: () => to("fetchingNextPage"),
				};
			},
			fetchingPreviousPage(_, to) {
				const tail = logStore().getTail();
				const cursor = tail
					? ({
							direction: "backward",
							cursorTime: tail.value.timestamp,
							cursorSequenceNumber: tail.value.sequenceNumber,
						} satisfies LogPageCursor)
					: null;
				fetchPage(cursor).then((page) => {
					logStore().insertMany(page);
				});
				return {
					reset: () => {
						to("initial");
					},
				};
			},
			fetchingNextPage(_, to) {
				return {
					reset: () => {
						to("initial");
					},
				};
			},
		},
	});

	createEffect(
		on(props, () => {
			machine.value.reset();
		}),
	);

	return {
		get logs() {
			return logStore();
		},
		state: machine,
	};
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

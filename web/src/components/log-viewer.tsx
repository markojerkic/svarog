
import { createEffect, createMemo, onMount } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import {
	createLogQueryOptions,
	insertLogLine,
	type LogLine,
	type LogPageCursor,
} from "@/lib/store/query";
import {
	createInfiniteQuery,
	type InfiniteData,
	useQueryClient,
} from "@tanstack/solid-query";
import { createMachine } from "@solid-primitives/state-machine";
import { newLogLineListener } from "@/lib/store/connection";
import { ScrollArea } from "./scroll-area";

export const LogViewer = (props: {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
}) => {
	// LOGS
	const queryClient = useQueryClient();
	const query = createInfiniteQuery(() => createLogQueryOptions(() => props));
	const logs = createMemo(() => {
		if (!query.data) {
			return [];
		}

		return query.data.pages.flat();
	});

	const logCount = () => logs().length;

	const queryState = createMachine<{
		idle: { value: { isFetching: () => void } };
		fetchingPreviousPage: { value: { isDone: () => void } };
		isDoneFetchingPreviousPage: { value: { isFetching: () => void } };
	}>({
		initial: "idle",
		states: {
			idle(_input, to) {
				const isFetching = () => {
					to("fetchingPreviousPage");
				};

				return { isFetching };
			},
			fetchingPreviousPage(_input, to) {
				const isDone = () => {
					to("isDoneFetchingPreviousPage");
					console.log("Scrolling to index", query.data?.pages[0].length);
					//virtualizer.scrollToIndex(query.data?.pages[0].length ?? 0);
					console.error("Treba scroll");
				};

				return { isDone };
			},
			isDoneFetchingPreviousPage(_input, to) {
				const isFetching = () => {
					to("fetchingPreviousPage");
				};

				return { isFetching };
			},
		},
	});

	createEffect(() => {
		if (query.isFetchingPreviousPage) {
			if (
				queryState.type === "isDoneFetchingPreviousPage" ||
				queryState.type === "idle"
			) {
				queryState.value.isFetching();
			}
		} else if (queryState.type === "fetchingPreviousPage") {
			queryState.value.isDone();
		}
	});

	onMount(() => {
		const unsub = newLogLineListener((line) => {
			const queryKey = createLogQueryOptions(() => props).queryKey;
			queryClient.setQueryData(
				queryKey,
				(oldData: InfiniteData<LogLine[], LogPageCursor | undefined>) => {
					return insertLogLine(oldData, line);
				},
			);
		});

		return () => unsub();
	});

	return (
		<ScrollArea
			fetchPrevious={() => {
				console.log("Daj previous");
			}}
			fetchNext={() => {
				console.log("Daj next");
			}}
			itemCount={logCount()}
		>
			{(virtualItem) => {
				const item = logs()[virtualItem.index];
				const color = useInstanceColor(item.client.ipAddress);

				return (
					<pre
						data-index={virtualItem.index}
						class={"border-l-4 pl-2 text-black hover:border-l-8"}
						style={{
							"--tw-border-opacity": 1,
							"border-left-color": color(),
						}}
						ref={(_el) =>
							queueMicrotask(() => {
								//virtualizer.measureElement(el)
								console.warn("Nezz triba li ovo popraviti");
							})
						}
					>
						{item.content}
					</pre>
				);
			}}
		</ScrollArea>
	);
};

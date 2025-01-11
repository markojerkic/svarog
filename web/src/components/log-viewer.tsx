import { createEffect, createMemo, on, onMount } from "solid-js";
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
import { useScrollEvent } from "@/lib/hooks/use-scroll-event";

export const LogViewer = (props: {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
}) => {
	const scrollEventBus = useScrollEvent();
	const queryClient = useQueryClient();
	const query = createInfiniteQuery(() => createLogQueryOptions(() => props));
	const logs = createMemo(() => {
		if (!query.data) {
			return [];
		}

		return query.data.pages.flat();
	});

	const logCount = () => logs().length;

	const machine = createMachine<{
		idle: {
			to: "fetchingPreviousPage" | "fetchingNextPage";
		};
		fetchingNextPage: {
			to: "idle";
		};
		fetchingPreviousPage: {
			to: "idle";
		};
	}>({
		initial: "idle",
		states: {
			idle() {},
			fetchingPreviousPage() {},
			fetchingNextPage() {},
		},
	});

	createEffect(
		on(
			() => query.status,
			() => {
				if (machine.type === "idle") {
					if (query.isFetchingPreviousPage) {
						machine.to("fetchingPreviousPage");
					}
					if (query.isFetchingNextPage) {
						machine.to("fetchingNextPage");
					}
				} else {
					// Was fetching previous page, now it's done
					if (machine.type === "fetchingPreviousPage") {
						scrollEventBus.scrollToIndex(query.data?.pages[0].length ?? 0);
					}

					machine.to("idle");
				}
			},
		),
	);

	onMount(() => {
		scrollEventBus.scrollToBottom();

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
		<>
			<ScrollArea
				fetchPrevious={() => {
					query.fetchPreviousPage();
				}}
				fetchNext={() => {
					query.fetchNextPage();
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
						>
							{item.content}
						</pre>
					);
				}}
			</ScrollArea>
		</>
	);
};

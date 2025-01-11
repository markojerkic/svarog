import { onMount, Show } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import {
	createLogQueryOptions,
	insertLogLine,
	type LogLine,
	type LogPageCursor,
} from "@/lib/store/query";
import { type InfiniteData, useQueryClient } from "@tanstack/solid-query";
import { newLogLineListener } from "@/lib/store/connection";
import { ScrollArea } from "./scroll-area";
import { useScrollEvent } from "@/lib/hooks/use-scroll-event";
import { useLogStore } from "@/lib/hooks/use-log-store";

export const LogViewer = (props: {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
}) => {
	const scrollEventBus = useScrollEvent();
	const queryClient = useQueryClient();
	const logStore = useLogStore(() => ({
		clientId: props.clientId,
		selectedInstances: props.selectedInstances,
		searchQuery: props.searchQuery,
	}));

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
			<p>Log machine state: {logStore.state.type}</p>
			<ScrollArea
				fetchPrevious={() => {
					console.log("want to fetch previous");
					if (logStore.state.type === "idle") {
						console.log("fetching previous");
						logStore.state.value.fetchPreviousPage();
					} else {
						console.log(
							"not fetching previous because state is not idle",
							logStore.state.type,
						);
					}
				}}
				fetchNext={() => {
					if (logStore.state.type === "idle") {
						logStore.state.value.fetchNextPage();
					}
				}}
				itemCount={logStore.logs.size}
			>
				{(virtualItem) => {
					const item = logStore.logs.get(virtualItem.index); //logs()[virtualItem.index];

					return (
						<Show when={item} keyed>
							{(item) => (
								<pre
									data-index={virtualItem.index}
									class={"border-l-4 pl-2 text-black hover:border-l-8"}
									style={{
										"--tw-border-opacity": 1,
										"border-left-color": useInstanceColor(
											item.client.ipAddress,
										),
									}}
								>
									{item.content}
								</pre>
							)}
						</Show>
					);
				}}
			</ScrollArea>
		</>
	);
};

import { onMount, Show } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
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
	const logStore = useLogStore(() => ({
		clientId: props.clientId,
		selectedInstances: props.selectedInstances,
		searchQuery: props.searchQuery,
	}));

	onMount(() => {
		scrollEventBus.scrollToBottom();

		const unsub = newLogLineListener((lines) => {
			logStore.logs.insertMany(lines);
		});

		return () => {
			unsub();
		};
	});

	return (
		<>
			<ScrollArea
				fetchPrevious={() => {
					if (logStore.state.type === "idle") {
						logStore.state.value.fetchPreviousPage();
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
					const item = logStore.logs.get(virtualItem.index);

					return (
						<Show when={item} keyed>
							{(item) => (
								<pre
									data-index={virtualItem.index}
									data-log-line
									class={"border-l-4 pl-2 text-black"}
									style={{
										"--instance-color": useInstanceColor(item.client.ipAddress),
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

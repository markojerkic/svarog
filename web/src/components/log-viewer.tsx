import { onMount, Show } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import { newLogLineListener } from "@/lib/store/connection";
import { ScrollArea } from "./scroll-area";
import { useLogStore } from "@/lib/hooks/use-log-store";
import {
	ContextMenu,
	ContextMenuContent,
	ContextMenuItem,
	ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { toast } from "solid-sonner";

export const LogViewer = (props: {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
	selectedLogLineId?: string;
}) => {
	const logStore = useLogStore(() => ({
		clientId: props.clientId,
		selectedInstances: props.selectedInstances,
		searchQuery: props.searchQuery,
		selectedLogLineId: props.selectedLogLineId,
	}));

	onMount(() => {
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
								<ContextMenu>
									<ContextMenuTrigger
										as="pre"
										data-log-line
										data-line-is-selected={item.id === props.selectedLogLineId}
										class={"border-l-4 pl-2 text-black"}
										style={{
											"--instance-color": useInstanceColor(
												item.client.ipAddress,
											),
										}}
									>
										{item.content}
									</ContextMenuTrigger>
									<ContextMenuOptions
										logLineId={item.id}
										clientId={item.client.clientId}
										instanceId={item.client.ipAddress}
									/>
								</ContextMenu>
							)}
						</Show>
					);
				}}
			</ScrollArea>
		</>
	);
};

const ContextMenuOptions = (props: {
	logLineId: string;
	clientId: string;
	instanceId: string;
}) => {
	const copyLogLineAddress = () => {
		const currentDomain = window.location.origin;
		const logLineUrl = `${currentDomain}/logs/${props.clientId}?logLine=${props.logLineId}&instances=${props.instanceId}`;
		navigator.clipboard.writeText(logLineUrl);
		toast.success("Log line address copied to clipboard");
	};

	return (
		<ContextMenuContent class="w-64">
			<ContextMenuItem inset onSelect={() => copyLogLineAddress()}>
				Copy log line address
			</ContextMenuItem>
		</ContextMenuContent>
	);
};

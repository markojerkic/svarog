import { createVirtualizer } from "@tanstack/solid-virtual";
import {
	For,
	Show,
	createEffect,
	createSignal,
	on,
	onCleanup,
	onMount,
} from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import {
	fetchLogPage,
	type LogPageCursor,
	type LogLine,
} from "@/lib/store/query";
import { createInfiniteQuery } from "@tanstack/solid-query";

const LogViewer = (props: {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
}) => {
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let logsRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let topRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let bottomRef: HTMLDivElement | undefined = undefined;
	const windowHeight = useWindowHeight();
	const scrollViewerHeight = () => `${Math.ceil(windowHeight() * 0.8)}px`;

	const [scrollPreservationIndex, _setScrollPreservationIndex] =
		createSignal<number>(300);

	// LOGS
	const [logs, setLogs] = createSignal<LogLine[]>([]);
	const query = createInfiniteQuery(() => ({
		queryKey: [
			"logs",
			props.clientId,
			props.selectedInstances,
			props.searchQuery,
		],
		queryFn: async ({ pageParam, signal }) => {
			return fetchLogPage(
				props.clientId,
				{
					selectedInstances: props.selectedInstances,
					search: props.searchQuery,
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

	const logCount = () => logs().length;

	const virtualizer = createVirtualizer({
		get count() {
			return logCount();
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
		overscan: 5,
	});

	const observer = new IntersectionObserver((entries) => {
		for (const entry of entries) {
			if (entry.isIntersecting) {
				if (entry.target.id === "bottom" && !query.isFetchingNextPage) {
					console.log("fetchNextPage");
					query.fetchNextPage();
				} else if (entry.target.id === "top" && !query.isFetchingPreviousPage) {
					console.log("fetchPreviousPage");
					query.fetchPreviousPage();
				}
			}
		}
	});

	createEffect(
		on(logs, () => {
			// If loading previous page, preserve scroll position
			const index = Math.min(scrollPreservationIndex(), logs().length - 1);
			if (index !== -1) {
				const preservedLogLine = logs()[index];

				setLogs(query.data?.pages.flat() ?? []);

				const newIndex = logs().findIndex(
					(log) => log.id === preservedLogLine.id,
				);
				if (newIndex !== -1) {
					virtualizer.scrollToIndex(newIndex);
				}
			} else {
				setLogs(query.data?.pages.flat() ?? []);
			}
		}),
	);

	onMount(() => {
		if (topRef) {
			observer.observe(topRef);
		}
		if (bottomRef) {
			observer.observe(bottomRef);
		}
	});

	const scrollToBottom = () => {
		console.log("Scroll to bottom event");
		virtualizer.scrollToIndex(logs().length, { align: "end" });
		//setIsOnBottom();
	};

	const isLockedInBottom = () => false;

	onMount(() => {
		scrollToBottom();
	});

	const items = virtualizer.getVirtualItems();

	return (
		<div
			ref={logsRef}
			class="scrollbar-thin scrollbar-track-zinc-900 scrollbar-thumb-zinc-700 ml-4 rounded-l-md border border-black"
			style={{
				height: scrollViewerHeight(),
				width: "90vw%",
				"overflow-y": "auto",
			}}
		>
			<div
				style={{
					height: `${virtualizer.getTotalSize()}px`,
					width: "100%",
					position: "relative",
				}}
			>
				<ScrollToBottomButton
					scrollToBottom={scrollToBottom}
					isLockedInBottom={isLockedInBottom()}
				/>
				<div
					style={{
						position: "absolute",
						top: 0,
						left: 0,
						width: "100%",
						transform: `translateY(${items.length ? items[0].start : 0}px)`,
					}}
				>
					<div id="top" ref={topRef} />
					<For each={virtualizer.getVirtualItems()}>
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
									ref={(el) =>
										queueMicrotask(() => virtualizer.measureElement(el))
									}
								>
									{item.content}
								</pre>
							);
						}}
					</For>
					<div id="bottom" class="my-[-2rem]" ref={bottomRef} />
				</div>
			</div>
		</div>
	);
};

const ScrollToBottomButton = (props: {
	isLockedInBottom: boolean;
	scrollToBottom: () => void;
}) => {
	return (
		<Show when={!props.isLockedInBottom}>
			<button
				type="button"
				id="scroll-to-bottom"
				class="fixed right-4 bottom-4 z-[1000] flex size-10 cursor-pointer rounded-full bg-red-800 hover:bg-red-700"
				onClick={props.scrollToBottom}
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					fill="none"
					viewBox="0 0 24 24"
					stroke-width="2.5"
					class="m-auto size-6 stroke-white"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M19.5 13.5 12 21m0 0-7.5-7.5M12 21V3"
					/>
				</svg>
			</button>
		</Show>
	);
};

const useWindowHeight = () => {
	const [height, setHeight] = createSignal(window.innerHeight);

	onMount(() => {
		const handleResize = () => setHeight(window.innerHeight);
		window.addEventListener("resize", handleResize);
		onCleanup(() => window.removeEventListener("resize", handleResize));
	});

	return height;
};

export const createLogViewer = () => {
	const scrollToBottom = () => {
		const scrollToBottomEvent = new Event("scroll-to-bottom");
		dispatchEvent(scrollToBottomEvent);
	};

	return [LogViewer, scrollToBottom] as const;
};

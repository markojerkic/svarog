import { createVirtualizer } from "@tanstack/solid-virtual";
import { For, createEffect, createMemo, onMount } from "solid-js";
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
import { useWindowHeight } from "@/lib/hooks/use-window-height";
import { ScrollToBottomButton } from "./scroll-to-bottom-button";

export const LogViewer = (props: {
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

	const isLockedInBottom = () => false;

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

	const virtualizer = createVirtualizer({
		get count() {
			return logCount();
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
		overscan: 5,
	});
	const scrollToBottom = () => {
		virtualizer.scrollToIndex(logs().length, { align: "end" });
	};

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
					virtualizer.scrollToIndex(query.data?.pages[0].length ?? 0);
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
		scrollToBottom();
	});

	onMount(() => {
		observer.observe(topRef!);
		observer.observe(bottomRef!);
	});

	onMount(() => {
		const unsub = newLogLineListener((line) => {
			console.log("New line", line);
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

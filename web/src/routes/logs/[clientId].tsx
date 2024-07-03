import { type RouteDefinition, useParams } from "@solidjs/router";
import {
	type FetchInfiniteQueryOptions,
	createInfiniteQuery,
	useQueryClient,
} from "@tanstack/solid-query";
import { VirtualItem, createVirtualizer } from "@tanstack/solid-virtual";
import { For, Show, createEffect, onCleanup, onMount } from "solid-js";
import { createInfiniteScrollObserver } from "~/lib/infinite-scroll";

type LogLine = {
	id: string;
	timestamp: number;
	content: string;
};

const getLogsQueryOptions = (clientId: string) =>
	({
		queryKey: ["logs", clientId],
		queryFn: async (params) => {
			const response = await fetch(params.pageParam as string);
			const data = response.json() as Promise<LogLine[]>;

			return data;
		},
		initialPageParam: buildUrl(clientId),
	}) satisfies FetchInfiniteQueryOptions;

const buildBaseUrl = (clientId: string) =>
	`http://localhost:1323/api/v1/logs/${clientId}`;

const buildUrl = (
	clientId: string,
	params?: {
		cursorId: string;
		cursorTime: number;
		direction: "forward" | "backward";
	},
) => {
	let url = buildBaseUrl(clientId);
	if (params) {
		url += `?cursorId=${params.cursorId}&cursorTime=${params.cursorTime}&direction=${params.direction}`;
	}
	return url;
};

const getLogsPage = (queryClient: string) =>
	createInfiniteQuery(() => ({
		...getLogsQueryOptions(queryClient),
		getNextPageParam: (lastPage) => {
            if (!lastPage) return null;
			const last = lastPage[lastPage.length - 1];
			if (!last) {
				return null;
			}
			return buildUrl(queryClient, {
				cursorId: last.id,
				cursorTime: last.timestamp,
				direction: "forward",
			});
		},
		getPreviousPageParam: (lastPage) => {
            if (!lastPage) return null;
			const first = lastPage[0];
			if (!first) {
				return null;
			}
			return buildUrl(queryClient, {
				cursorId: first.id,
				cursorTime: first.timestamp,
				direction: "backward",
			});
		},
	}));

export const route = {
	load: ({ params }) => {
		const clientId = params.clientId;
		const queryClient = useQueryClient();

		queryClient.setQueryData(
			["logs", clientId],
			(data: { pages: unknown[][]; pageParams: string[] }) => {
				if (!data || data.pages.length === 0) {
					return {
						pages: [],
						pageParams: [],
					};
				}

				const flattened = data.pages.flat() as LogLine[];
				const lastPageParam = buildUrl(clientId, {
					cursorId: flattened[0].id,
					cursorTime: flattened[0].timestamp,
					direction: "backward",
				});
				//const nextPageParam = buildUrl(clientId, {
				//    cursorId: flattened[flattened.length - 1].id,
				//    cursorTime: flattened[flattened.length - 1].timestamp,
				//    direction: "forward",
				//})
				const next = flattened.pop();

				return {
					pages: [flattened, [next]],
					pageParams: [buildBaseUrl(clientId), lastPageParam],
				};
			},
		);

		return queryClient.fetchInfiniteQuery(getLogsQueryOptions(clientId));
	},
} satisfies RouteDefinition;

export default () => {
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let logsRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let topRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let bottomRef: HTMLDivElement | undefined = undefined;

	const clientId = useParams<{ clientId: string }>().clientId;
	const logs = getLogsPage(clientId);
	const logsOrEmpty = () => logs.data?.pages.flat().toReversed() ?? [];
	const logCount = () => logsOrEmpty().length;

	const virtualizer = createVirtualizer({
		get count() {
			return logCount();
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
		overscan: 5,
	});

	//let hasScrolledToBottom = false;

	const observer = createInfiniteScrollObserver(logs);
	onMount(() => {
		if (topRef) {
			observer.observe(topRef);
		}
		if (bottomRef) {
			observer.observe(bottomRef);
		}
	});
	onCleanup(() => {
		observer.disconnect();
	});

	let wasFetchingPreviousPage = false;
	createEffect(() => {
		if (logs.isFetchingPreviousPage) {
			wasFetchingPreviousPage = true;
		} else if (
			wasFetchingPreviousPage &&
			logs.data &&
			virtualizer.isScrolling
		) {
			wasFetchingPreviousPage = false;
			// if virtulizer is currently at the top, scroll to the top
			const offset = logs.data.pages[0].length;
			console.log("Scrolling to", offset);
			virtualizer.scrollToIndex(offset, { align: "start" });
		}
	});

	//onMount(() => {
	//	if (logs.data && logs.data.pages.length === 1 && !hasScrolledToBottom) {
	//		console.log("Mounting to end");
	//		hasScrolledToBottom = true;
	//		// scroll to bottom
	//		virtualizer.scrollToIndex(logCount() - 3, { align: "end" });
	//	}
	//});

	const handleScroll = (e: WheelEvent) => {
		e.preventDefault();
		console.log("dude");
		const currentTarget = e.currentTarget as HTMLElement;

		if (currentTarget) {
			currentTarget.scrollTop -= e.deltaY;
		}
	};
	onMount(() => {
		logsRef?.addEventListener("wheel", handleScroll, {
			passive: false,
		});
	});
	onCleanup(() => {
		logsRef?.removeEventListener("wheel", handleScroll);
	});

	return (
		<div>
			<button
				class="bg-green-500 p-1 rounded-md text-white"
				onClick={() => logs.fetchPreviousPage()}
				type="button"
				disabled={!logs.hasPreviousPage && !logs.isFetchingPreviousPage}
			>
				<Show
					when={logs.isFetchingPreviousPage}
					fallback={"Fetch previous page"}
				>
					<span class="animate-bounce">...</span>
				</Show>
			</button>

			<pre>Total: {logCount()}</pre>

			<div
				ref={logsRef}
				style={{
					height: `500px`,
					width: `100%`,
					overflow: "auto",
					transform: "scaleY(-1)",
				}}
			>
				<div
					style={{
						height: `${virtualizer.getTotalSize()}px`,
						width: "100%",
						position: "relative",
					}}
				>
					<div id="bottom" ref={bottomRef} />
					<For each={virtualizer.getVirtualItems()}>
						{(virtualItem) => {
							const item = () => logsOrEmpty()[virtualItem.index].content;
							return (
								<div
									style={{
										position: "absolute",
										top: 0,
										left: 0,
										width: "100%",
										height: `${virtualItem.size}px`,
										transform: `translateY(${virtualItem.start}px) scaleY(-1)`,
									}}
								>
									<pre class="text-white">
										{`${virtualItem.index} `}
										{item()}
									</pre>
								</div>
							);
						}}
					</For>
					<div id="top" ref={topRef} />
				</div>
			</div>

			<button
				class="bg-blue-500 p-1 rounded-md text-white"
				onClick={() => logs.fetchNextPage()}
				type="button"
				disabled={!logs.hasNextPage}
			>
				Fetch next
			</button>
		</div>
	);
};

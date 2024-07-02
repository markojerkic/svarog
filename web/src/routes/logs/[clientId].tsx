import { type RouteDefinition, useParams } from "@solidjs/router";
import {
	type FetchInfiniteQueryOptions,
	createInfiniteQuery,
	useQueryClient,
} from "@tanstack/solid-query";
import { createVirtualizer } from "@tanstack/solid-virtual";
import { For, onMount } from "solid-js";
import { createInfiniteScrollObserver } from "~/lib/infinite-scroll";

type LogLine = {
	id: string;
	timestamp: number;
	content: string;
};

const getLogsQueryOptions = (queryClient: string) =>
	({
		queryKey: ["logs", queryClient],
		queryFn: async (params) => {
			const response = await fetch(params.pageParam as string);
			return response.json() as Promise<LogLine[]>;
		},
		initialPageParam: buildUrl(queryClient),
	}) satisfies FetchInfiniteQueryOptions;

const buildUrl = (
	clieentId: string,
	params?: {
		cursorId: string;
		cursorTime: number;
		direction: "forward" | "backward";
	},
) => {
	let url = `http://localhost:1323/api/v1/logs/${clieentId}`;
	if (params) {
		url += `?cursorId=${params.cursorId}&cursorTime=${params.cursorTime}&direction=${params.direction}`;
	}
	return url;
};

const getLogsPage = (queryClient: string) =>
	createInfiniteQuery(() => ({
		...getLogsQueryOptions(queryClient),
		getNextPageParam: (lastPage) => {
			const last = lastPage[lastPage.length - 1];
			if (!last) {
				console.log("No next page");
				return null;
			}
			console.log("Fetcing next page");
			return buildUrl(queryClient, {
				cursorId: last.id,
				cursorTime: last.timestamp,
				direction: "forward",
			});
		},
		getPreviousPageParam: (lastPage) => {
			const first = lastPage[0];
			if (!first) {
				console.log("No previous page");
				return null;
			}
			console.log("Fetcing previous page");
			return buildUrl(queryClient, {
				cursorId: first.id,
				cursorTime: first.timestamp,
				direction: "backward",
			});
		},
		staleTime: 1000,
	}));

export const route = {
	load: ({ params }) => {
		const clientId = params.clientId;
		const queryClient = useQueryClient();
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
	const logsOrEmpty = () => logs.data?.pages.flat() ?? [];
	const logCount = () => logsOrEmpty().length;
	createInfiniteScrollObserver(logs, topRef, bottomRef);

	const virtualizer = createVirtualizer({
		get count() {
			return logCount();
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
	});

	onMount(() => {
		const offset = (logs.data?.pages ?? [[]])[0].length * 25;
		virtualizer.scrollOffset = offset;
		virtualizer.calculateRange();
		virtualizer.scrollToOffset(offset, { align: "start" });
	});

	return (
		<>
			<div
				ref={logsRef}
				style={{
					height: "90vh",
					width: "100%",
					overflow: "auto",
				}}
			>
				<div
					style={{
						height: `${virtualizer.getTotalSize()}px`,
						width: "100%",
						position: "relative",
					}}
				>
					<div id="top" ref={topRef} />
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
										transform: `translateY(${virtualItem.start - virtualizer.options.scrollMargin}px)`,
									}}
								>
									<pre class="text-white">{item()}</pre>
								</div>
							);
						}}
					</For>
					<div id="bottom" ref={bottomRef} />
				</div>
			</div>
		</>
	);
};

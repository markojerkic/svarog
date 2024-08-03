import { type RouteDefinition, useParams } from "@solidjs/router";
import { createVirtualizer } from "@tanstack/solid-virtual";
import { For, Show, createEffect, onCleanup, onMount } from "solid-js";
import { createInfiniteScrollObserver } from "~/lib/infinite-scroll";
import { createLogSubscription } from "~/lib/store/connection";
import { createLogQuery } from "~/lib/store/log-store";

export const route = {
	load: async ({ params }) => {
		const clientId = params.clientId;

		const logData = createLogQuery(clientId);
		return await logData.fetchPreviousPage();
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
	const logs = createLogQuery(clientId);
	const logCount = () => logs.logStore.size;

	createLogSubscription(clientId, logs.logStore);

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
		if (logs.isPreviousPageLoading()) {
			wasFetchingPreviousPage = true;
		} else if (
			wasFetchingPreviousPage &&
			logs.logStore &&
			virtualizer.isScrolling
		) {
			wasFetchingPreviousPage = false;
			// if virtulizer is currently at the top, scroll to the top
			const offset = logs.lastLoadedPageSize() - 1;
			virtualizer.scrollToIndex(offset, { align: "start" });
		}
	});

	onMount(() => {
		virtualizer.scrollToIndex(logCount(), { align: "end" });
	});

	const items = virtualizer.getVirtualItems();

	return (
		<div>
			<button
				class="bg-green-500 p-1 rounded-md text-white"
				onClick={() => logs.fetchPreviousPage()}
				type="button"
				disabled={logs.isPreviousPageLoading()}
			>
				<Show
					when={logs.isPreviousPageLoading()}
					fallback={"Fetch previous page"}
				>
					<span class="animate-bounce">...</span>
				</Show>
			</button>

			<pre>Total: {logCount()}</pre>

			<div
				ref={logsRef}
				style={{ height: "80vh", width: "100%", "overflow-y": "auto" }}
			>
				<div
					style={{
						height: `${virtualizer.getTotalSize()}px`,
						width: "100%",
						position: "relative",
					}}
				>
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
								const item = () => {
									logs.logStore.size;
									return logs.logStore.get(virtualItem.index)?.content;
								};

								return (
									<div
										data-index={virtualItem.index}
										ref={(el) =>
											queueMicrotask(() => virtualizer.measureElement(el))
										}
									>
										<pre class="text-white">{item()}</pre>
									</div>
								);
							}}
						</For>
						<div id="bottom" ref={bottomRef} />
					</div>
				</div>
			</div>

			<button
				class="bg-blue-500 p-1 rounded-md text-white"
				onClick={() => logs.fetchNextPage()}
				type="button"
				disabled={logs.isNextPageLoading()}
			>
				Fetch next
			</button>
		</div>
	);
};

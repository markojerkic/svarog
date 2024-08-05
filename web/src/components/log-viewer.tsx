import { createVirtualizer } from "@tanstack/solid-virtual";
import { For, createEffect, onMount } from "solid-js";
import { createInfiniteScrollObserver } from "~/lib/infinite-scroll";
import type { CreateLogQueryResult } from "~/lib/store/log-store";

type LogViewerProps = {
	logsQuery: CreateLogQueryResult;
};
const LogViewer = (props: LogViewerProps) => {
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let logsRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let topRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let bottomRef: HTMLDivElement | undefined = undefined;

	const logs = props.logsQuery;
	const logCount = () => logs.logStore.size;

	const virtualizer = createVirtualizer({
		get count() {
			return logCount();
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
		overscan: 5,
	});

	const [observer, isLockedInBottom] = createInfiniteScrollObserver(logs);
	onMount(() => {
		if (topRef) {
			observer.observe(topRef);
		}
		if (bottomRef) {
			observer.observe(bottomRef);
		}
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
		scrollToBottom();
	});

	const items = virtualizer.getVirtualItems();

	const scrollToBottom = () => {
		if (isLockedInBottom()) {
			virtualizer.scrollToIndex(logs.logStore.size, { align: "end" });
		}
	};
	onMount(() => {
		addEventListener("scroll-to-bottom", scrollToBottom);

		return () => {
			removeEventListener("scroll-to-bottom", scrollToBottom);
		};
	});

	return (
		<div>
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
		</div>
	);
};

export const createLogViewer = () => {
	const scrollToBottom = () => {
		const scrollToBottomEvent = new Event("scroll-to-bottom");
		dispatchEvent(scrollToBottomEvent);
	};

	return [LogViewer, scrollToBottom] as const;
};

import { createVirtualizer } from "@tanstack/solid-virtual";
import { For, Show, createEffect, onCleanup, onMount } from "solid-js";
import { b } from "vitest/dist/suite-IbNSsUWN.js";
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

	const [observer, isLockedInBottom, setIsOnBottom] =
		createInfiniteScrollObserver(logs);
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

	const scrollToBottom = () => {
		virtualizer.scrollToIndex(logs.logStore.size, { align: "end" });
		setIsOnBottom();
	};

	const scrollToBottomIfLocked = () => {
		if (isLockedInBottom()) {
			scrollToBottom();
		}
	};

	onMount(() => {
		scrollToBottom();
	});

	const items = virtualizer.getVirtualItems();

	addEventListener("scroll-to-bottom", scrollToBottomIfLocked);
	onCleanup(() =>
		removeEventListener("scroll-to-bottom", scrollToBottomIfLocked),
	);

	return (
		<div>
			<pre class="text-white p-4">
				Log Viewer is locked in bottom: {isLockedInBottom() ? "Je" : "Nije"}
			</pre>
			<div
				ref={logsRef}
				class="scrollbar-thin scrollbar-track-zinc-900 scrollbar-thumb-zinc-700"
				style={{ height: "70vh", width: "100%", "overflow-y": "auto" }}
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
						<div id="bottom" class="my-[-2rem]" ref={bottomRef} />
					</div>
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
				class="size-10 rounded-full bg-red-800 flex fixed bottom-4 right-4 cursor-pointer hover:bg-red-700"
				onClick={props.scrollToBottom}
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					fill="none"
					viewBox="0 0 24 24"
					stroke-width="2.5"
					class="size-6 m-auto stroke-white"
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

export const createLogViewer = () => {
	const scrollToBottom = () => {
		const scrollToBottomEvent = new Event("scroll-to-bottom");
		dispatchEvent(scrollToBottomEvent);
	};

	return [LogViewer, scrollToBottom] as const;
};

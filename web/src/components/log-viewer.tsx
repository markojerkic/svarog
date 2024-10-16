import { createVirtualizer } from "@tanstack/solid-virtual";
import {
	For,
	Show,
	createEffect,
	createSignal,
	onCleanup,
	onMount,
} from "solid-js";
import { useInstanceColor } from "~/lib/hooks/instance-color";
import { createInfiniteScrollObserver } from "~/lib/infinite-scroll";
import type { CreateLogQueryResult } from "~/lib/store/query";

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
	const windowHeight = useWindowHeight();
	const scrollViewerHeight = () => `${Math.ceil(windowHeight() * 0.8)}px`;

	createEffect(() => {
		console.log("Window height", scrollViewerHeight());
	});

	const logs = () => props.logsQuery.data;
	const logCount = () => props.logsQuery.logCount;

	const virtualizer = createVirtualizer({
		get count() {
			const lc = logCount();
			console.log("logCount", lc);
			return lc;
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
		overscan: 5,
	});

	const [observer, isLockedInBottom, setIsOnBottom] =
		createInfiniteScrollObserver(props.logsQuery);

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
		if (props.logsQuery.queryDetails.isFetchingPreviousPage) {
			wasFetchingPreviousPage = true;
		} else if (wasFetchingPreviousPage && virtualizer.isScrolling) {
			wasFetchingPreviousPage = false;
			// if virtulizer is currently at the top, scroll to the top
			const offset = props.logsQuery.lastLoadedPageSize() - 1;
			virtualizer.scrollToIndex(offset, { align: "start" });
		}
	});

	const scrollToBottom = () => {
		console.log("Scroll to bottom event");
		virtualizer.scrollToIndex(logs().size, { align: "end" });
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
							const item = () => {
								const item = logs().get(virtualItem.index);
								const content = item?.content;
								const instance = item?.client.ipAddress ?? "";
								return { content, instance };
							};
							const color = useInstanceColor(item().instance);

							return (
								<div
									data-index={virtualItem.index}
									ref={(el) =>
										queueMicrotask(() => virtualizer.measureElement(el))
									}
								>
									<pre
										class={"border-l-4 pl-2 text-black hover:border-l-8"}
										style={{
											"--tw-border-opacity": 1,
											"border-left-color": color(),
										}}
									>
										{item().content}
									</pre>
								</div>
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

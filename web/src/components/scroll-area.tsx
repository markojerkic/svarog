import { useWindowHeight } from "@/lib/hooks/use-window-height";
import { createViewportObserver } from "@solid-primitives/intersection-observer";
import { type VirtualItem, createVirtualizer } from "@tanstack/solid-virtual";
import { For, type JSXElement, createSignal, onMount } from "solid-js";
import { ScrollToBottomButton } from "./scroll-to-bottom-button";

type ScrollAreaProps = {
	itemCount: number;
	fetchNext: () => void;
	fetchPrevious: () => void;
	children: (virtualItem: VirtualItem) => JSXElement;
};

export const ScrollArea = (props: ScrollAreaProps) => {
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let logsRef: HTMLDivElement | undefined = undefined;
	const windowHeight = useWindowHeight();
	const scrollViewerHeight = () => `${Math.ceil(windowHeight() * 0.8)}px`;
	const [isLockedInBottom, _setIsLockedInBotton] = createSignal(false);

	const virtualizer = createVirtualizer({
		get count() {
			return props.itemCount;
		},
		estimateSize: () => 25,
		getScrollElement: () => logsRef ?? null,
		overscan: 5,
	});
	const scrollToBottom = () => {
		virtualizer.scrollToIndex(props.itemCount, { align: "end" });
	};

	// @ts-expect-error used in directive
	// biome-ignore lint/correctness/noUnusedVariables: used in directive
	const [intersectionObserver] = createViewportObserver();

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
					<div
						id="top"
						use:intersectionObserver={() => {
							console.log("to top");
							props.fetchPrevious();
						}}
					/>
					<For each={virtualizer.getVirtualItems()}>
						{(virtualItem) => props.children(virtualItem)}
					</For>
					<div
						id="bottom"
						class="my-[-2rem]"
						use:intersectionObserver={() => {
							console.log("to bottom");
							props.fetchNext();
						}}
					/>
				</div>
			</div>
		</div>
	);
};

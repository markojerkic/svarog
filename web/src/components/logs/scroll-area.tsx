import { useWindowHeight } from "@/lib/hooks/use-window-height";
import { createViewportObserver } from "@solid-primitives/intersection-observer";
import { type VirtualItem, createVirtualizer } from "@tanstack/solid-virtual";
import {
	For,
	type JSXElement,
	createEffect,
	createSignal,
	on,
	onMount,
} from "solid-js";
import { ScrollToBottomButton } from "@/components/logs/scroll-to-bottom-button";
import {
	useOnScrollToBottom,
	useOnScrollToIndex,
} from "@/lib/hooks/use-scroll-event";
import { newLogLineListener } from "@/lib/store/connection";
import { useDebounceEventListener } from "@/lib/hooks/debounce-event";

type ScrollAreaProps = {
	itemCount: number;
	fetchNext: () => void;
	fetchPrevious: () => void;
	children: (virtualItem: VirtualItem) => JSXElement;
};

export const ScrollArea = (props: ScrollAreaProps) => {
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let logsRef: HTMLDivElement | undefined = undefined;
	// biome-ignore lint/style/useConst: Needs to be let for solidjs to be able to track it
	let containerRef: HTMLDivElement | undefined = undefined;
	const windowHeight = useWindowHeight();
	const [scrollHeight, setScrollHeight] = createSignal("auto");

	const calculateScrollViewerHeight = () => {
		if (!containerRef) return "auto";

		const rect = (containerRef as HTMLDivElement).getBoundingClientRect();
		const topPosition = rect.top;
		// Add some bottom padding (adjust as needed)
		const bottomPadding = 20;
		// Calculate available height
		const availableHeight = windowHeight() - topPosition - bottomPadding;

		return `${Math.max(availableHeight, 200)}px`;
	};

	// Update height on window resize and component mount
	createEffect(
		on(windowHeight, () => {
			if (containerRef) {
				setScrollHeight(calculateScrollViewerHeight());
			}
		}),
	);

	const [isLockedInBottom, setIsLockedInBotton] = createSignal(true);
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
	useOnScrollToBottom(() => {
		scrollToBottom();
	});
	useOnScrollToIndex((index) => {
		virtualizer.scrollToIndex(index);
	});
	// @ts-expect-error used in directive
	// biome-ignore lint/correctness/noUnusedVariables: used in directive
	const [intersectionObserver] = createViewportObserver({
		rootMargin: "10px",
	});
	useDebounceEventListener(
		() => logsRef,
		"scroll",
		() => {
			const el = logsRef!;
			const isLockedInBotton =
				el.scrollTop + el.clientHeight >= el.scrollHeight - 100;
			setIsLockedInBotton(isLockedInBotton);
		},
		100,
	);
	onMount(() => {
		newLogLineListener(() => {
			if (isLockedInBottom()) {
				scrollToBottom();
			}
		});

		// Calculate initial height
		setScrollHeight(calculateScrollViewerHeight());
	});
	const items = virtualizer.getVirtualItems();
	return (
		<div ref={containerRef} class="relative">
			<div
				ref={logsRef}
				class="scrollbar-thin scrollbar-track-zinc-900 scrollbar-thumb-zinc-700 mx-auto rounded-l-md border border-black"
				style={{
					height: scrollHeight(),
					width: "95vw",
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
						isLockedInBottom={isLockedInBottom()}
						click={() => setIsLockedInBotton(true)}
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
							use:intersectionObserver={(el) => {
								if (el.intersectionRatio > 0.3) {
									props.fetchPrevious();
								}
							}}
						/>
						<For each={virtualizer.getVirtualItems()}>
							{(virtualItem) => props.children(virtualItem)}
						</For>
						<div
							id="bottom"
							class="my-[-2rem]"
							use:intersectionObserver={(el) => {
								if (el.intersectionRatio > 0.3) {
									props.fetchNext();
								}
							}}
						/>
					</div>
				</div>
			</div>
		</div>
	);
};

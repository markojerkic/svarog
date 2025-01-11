import { createEventBus, createEventHub } from "@solid-primitives/event-bus";
import { onMount } from "solid-js";

const scrollEventBus = createEventHub({
	scrollToBottom: createEventBus(),
	scrollToIndex: createEventBus<number>(),
});

export const useScrollEvent = () => {
	const scrollToBottom = () => {
		scrollEventBus.emit("scrollToBottom");
	};

	const scrollToIndex = (index: number) => {
		scrollEventBus.emit("scrollToIndex", index);
	};

	return { scrollToBottom, scrollToIndex };
};

export const useOnScrollToBottom = (callback: () => void) => {
	onMount(() => {
		const unsub = scrollEventBus.on("scrollToBottom", callback);
		return () => unsub();
	});
};

export const useOnScrollToIndex = (callback: (index: number) => void) => {
	onMount(() => {
		const unsub = scrollEventBus.on("scrollToIndex", callback);
		return () => unsub();
	});
};

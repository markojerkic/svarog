import { createEffect, type Accessor } from "solid-js";
import type { Timeout } from "./debounce";

export function useDebounceEventListener(
	target: Accessor<HTMLElement | undefined>,
	eventName: string,
	listener: (event: Event) => void,
	delay?: number,
) {
	createEffect(() => {
		let timeoutId: Timeout | undefined;
		const wrappedListener = (event: Event) => {
			if (timeoutId) {
				clearTimeout(timeoutId);
			}
			timeoutId = setTimeout(() => {
				listener(event);
			}, delay || 100);
		};
		target()?.addEventListener(eventName, wrappedListener);
		return () => {
			target().removeEventListener(eventName, wrappedListener);
		};
	});
}

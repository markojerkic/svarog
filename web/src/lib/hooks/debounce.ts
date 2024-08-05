import { createEffect, createSignal } from "solid-js";

export type Timeout = ReturnType<typeof setTimeout>;
export const useDebounce = (value: string, delay: number) => {
	let timeout: Timeout | undefined;
	const [debouncedValue, setDebouncedValue] = createSignal(value);

	createEffect(() => {
		if (timeout) {
			clearTimeout(timeout);
		}

		timeout = setTimeout(() => {
			setDebouncedValue(value);
		}, delay);
	});
	return debouncedValue;
};

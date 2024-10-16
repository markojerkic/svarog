import { type Accessor, createEffect } from "solid-js";

export const useWithPreviousValue = <T>(
	value: Accessor<T>,
	fn: (prev: T, curr: T) => void,
) => {
	let prev = value();
	createEffect(() => {
		const curr = value();
		fn(prev, curr);
		prev = curr;
	});
};

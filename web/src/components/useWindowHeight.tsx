import { createSignal, onMount, onCleanup } from "solid-js";

export const useWindowHeight = () => {
	const [height, setHeight] = createSignal(window.innerHeight);

	onMount(() => {
		const handleResize = () => setHeight(window.innerHeight);
		window.addEventListener("resize", handleResize);
		onCleanup(() => window.removeEventListener("resize", handleResize));
	});

	return height;
};

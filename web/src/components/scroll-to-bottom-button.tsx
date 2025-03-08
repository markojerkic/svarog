import { useScrollEvent } from "@/lib/hooks/use-scroll-event";
import { Show } from "solid-js";

export const ScrollToBottomButton = (props: {
	isLockedInBottom: boolean;
}) => {
	const scrollEventBus = useScrollEvent();

	return (
		<Show when={!props.isLockedInBottom}>
			<button
				type="button"
				id="scroll-to-bottom"
				class="fixed right-4 bottom-4 z-1000 flex size-10 cursor-pointer rounded-full bg-primary"
				onClick={() => scrollEventBus.scrollToBottom()}
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

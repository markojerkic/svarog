import { createSignal } from "solid-js";
import type { CreateLogQueryResult } from "~/lib/store/log-store";

export const createInfiniteScrollObserver = (query: CreateLogQueryResult) => {
	const created = new Date().getTime();
	const [isLockedInBottom, setIsLockedInBottom] = createSignal(true);

	const setIsOnBottom = () => {
		setIsLockedInBottom(true);
	};

	const observer = new IntersectionObserver((entries) => {
		let isBottomIntersecting = false;

		for (const entry of entries) {
			if (entry.isIntersecting) {
				if (entry.target.id === "bottom") {
					isBottomIntersecting = true;
				}

				if (entry.target.id === "bottom" && !query.query.isFetchingNextPage) {
					query.query.fetchNextPage();
				} else if (entry.target.id === "top" && !query.query.isFetchingPreviousPage) {
					query.query.fetchPreviousPage();
				}
			}
		}

		if (new Date().getTime() - created > 1000) {
			console.log("isBottomIntersecting", isBottomIntersecting);
			setIsLockedInBottom(isBottomIntersecting);
		}
	});

	return [observer, isLockedInBottom, setIsOnBottom] as const;
};

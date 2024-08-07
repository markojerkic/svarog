import { createSignal } from "solid-js";
import type { CreateLogQueryResult } from "~/lib/store/log-store";

export const createInfiniteScrollObserver = (query: CreateLogQueryResult) => {
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

				if (entry.target.id === "bottom" && !query.isNextPageLoading) {
					query.fetchNextPage();
				} else if (entry.target.id === "top" && !query.isPreviousPageLoading) {
					query.fetchPreviousPage();
				}
			}
		}

		setIsLockedInBottom(isBottomIntersecting);
	});

	return [observer, isLockedInBottom, setIsOnBottom] as const;
};

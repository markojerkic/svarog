import { createSignal } from "solid-js";
import type { CreateLogQueryResult } from "./store/query";

export const createInfiniteScrollObserver = (query: CreateLogQueryResult) => {
	let created = new Date().getTime();
	const [isLockedInBottom, setIsLockedInBottom] = createSignal(true);

	const setIsOnBottom = () => {
		setIsLockedInBottom(true);
		created = new Date().getTime();
	};

	const observer = new IntersectionObserver((entries) => {
		let isBottomIntersecting = false;

		for (const entry of entries) {
			if (entry.isIntersecting) {
				if (entry.target.id === "bottom") {
					isBottomIntersecting = true;
				}

				if (
					entry.target.id === "bottom" &&
					!query.queryDetails.isFetchingNextPage
				) {
					console.log("fetchNextPage");
					query.queryDetails.fetchNextPage();
				} else if (
					entry.target.id === "top" &&
					!query.queryDetails.isFetchingPreviousPage
				) {
					console.log("fetchPreviousPage");
					query.queryDetails.fetchPreviousPage();
				}
			}
		}

		if (new Date().getTime() - created > 1000) {
			setIsLockedInBottom(isBottomIntersecting);
		}
	});

	return [observer, isLockedInBottom, setIsOnBottom] as const;
};

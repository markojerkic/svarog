import type { CreateLogQueryResult } from "~/lib/store/log-store";

export const createInfiniteScrollObserver = (query: CreateLogQueryResult) => {
	const observer = new IntersectionObserver((entries) => {
		for (const entry of entries) {
			if (entry.isIntersecting) {
				if (entry.target.id === "bottom" && !query.isNextPageLoading) {
					query.fetchNextPage();
				} else if (entry.target.id === "top" && !query.isPreviousPageLoading) {
					query.fetchPreviousPage();
				}
			}
		}
	});
	return observer;
};

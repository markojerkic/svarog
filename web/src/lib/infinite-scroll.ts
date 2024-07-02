import type { CreateInfiniteQueryResult } from "@tanstack/solid-query";

export const createInfiniteScrollObserver = (
	query: CreateInfiniteQueryResult,
) => {
	const observer = new IntersectionObserver((entries) => {
		for (const entry of entries) {
			if (entry.isIntersecting) {
				if (
					entry.target.id === "bottom" &&
					query.hasNextPage &&
					!query.isFetchingNextPage
				) {
					query.fetchNextPage();
				} else if (
					entry.target.id === "top" &&
					query.hasPreviousPage &&
					!query.isFetchingPreviousPage
				) {
					query.fetchPreviousPage();
				}
			}
		}
	});
	return observer;
};

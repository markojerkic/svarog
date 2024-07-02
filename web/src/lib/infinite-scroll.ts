import type { CreateInfiniteQueryResult } from "@tanstack/solid-query";
import { onCleanup, onMount } from "solid-js";

export const createInfiniteScrollObserver  = (
	query: CreateInfiniteQueryResult,
	topRef: HTMLElement | undefined,
	bottomRef: HTMLElement | undefined,
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

	onMount(() => {
		if (topRef) {
			observer.observe(topRef);
		}
		if (bottomRef) {
			observer.observe(bottomRef);
		}
	});

	onCleanup(() => {
		observer.disconnect();
	});
};

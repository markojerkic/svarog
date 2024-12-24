import { debounce } from "@solid-primitives/scheduled";
import { useParams, useSearchParams } from "@solidjs/router";
import {
	ErrorBoundary,
	Show,
	Suspense,
	createEffect,
	createSignal,
} from "solid-js";
import { createLogViewer } from "~/components/log-viewer";
import { useSelectedInstances } from "~/lib/hooks/use-selected-instances";
import { createLogQuery, getInstances } from "~/lib/store/query";
import { useWithPreviousValue } from "~/lib/hooks/with-previous-value";
import { Instances, NOOP_WS_ACTIONS } from "~/components/instances";
import { createQuery } from "@tanstack/solid-query";

export default () => {
	const clientId = useParams().clientId;
	const selectedInstances = useSelectedInstances();

	const [search, setSearch] = createDebouncedSearch();

	const logsQuery = createLogQuery(() => clientId, selectedInstances, search);
	const instances = createQuery(() => ({
		queryKey: ["logs", "instances", clientId],
		queryFn: ({ signal }) => getInstances(clientId, signal),
		refetchOnWindowFocus: true,
	}));

	const logsStringified = () => {
		return logsQuery.logCount;
	};
	const [LogViewer, scrollToBottom] = createLogViewer();

	useWithPreviousValue(
		() => logsQuery.queryDetails.isFetched,
		(prev, curr) => {
			if (prev === false && curr === true) {
				scrollToBottom();
			}
		},
	);

	return (
		<div class="rounded-md border-white p-2">
			<p>Search term: {search()}</p>
			<div>
				<label for="search">Search</label>
				<input
					id="search"
					type="text"
					class="rounded-md border-white p-1 text-black"
					value={search()}
					onInput={(e) => setSearch(e.currentTarget.value)}
				/>
			</div>

			<div class="border-red p-2">
				<Show when={logsQuery.queryDetails.isFetchingNextPage}>
					<p class="animate-bounce">Loading next...</p>
				</Show>
				<Show when={logsQuery.queryDetails.isFetchingPreviousPage}>
					<p class="animate-bounce">Loading prev...</p>
				</Show>
				<div>Logs Num: {logsStringified()}</div>
				<ErrorBoundary fallback={<span class="bg-red-900 p-2">Error </span>}>
					<Suspense fallback={<div>Loading...</div>}>
						<Show when={instances.data}>
							{(instances) => (
								<Instances instances={instances()} actions={NOOP_WS_ACTIONS} />
							)}
						</Show>
					</Suspense>
				</ErrorBoundary>
				<LogViewer logsQuery={logsQuery} />
			</div>
		</div>
	);
};

const createDebouncedSearch = () => {
	const [searchParams, setSearchParams] = useSearchParams<{
		search?: string;
	}>();
	const [search, setSearch] = createSignal(searchParams.search ?? "");
	const trigger = debounce((value: string) => {
		setSearchParams({ search: value });
	}, 500);

	createEffect(() => {
		trigger(search());
	});
	const debouncedSearch = () => searchParams.search;

	return [debouncedSearch, setSearch] as const;
};

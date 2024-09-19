import { debounce } from "@solid-primitives/scheduled";
import { useParams, useSearchParams } from "@solidjs/router";
import { Show, createEffect, createSignal, on } from "solid-js";
import { createLogViewer } from "~/components/log-viewer";
import { createLogQuery } from "~/lib/store/log-store";

export default () => {
	const clientId = useParams().clientId;
	const [search, setSearch] = createDebouncedSearch();

	const l = createLogQuery(() => ({
		clientId,
		search: search(),
	}));

	const logsStringified = () => {
		return l.state.logStore.size;
	};
	const [LogViewer, scrollToBottom] = createLogViewer();

	createEffect(
		on(createDebouncedSearch, () => {
			l.fetchPreviousPage();
			scrollToBottom();
		}),
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
				<Show when={l.state.isNextPageLoading}>
					<p class="animate-bounce">Loading next...</p>
				</Show>
				<Show when={l.state.isPreviousPageLoading}>
					<p class="animate-bounce">Loading prev...</p>
				</Show>
				<div>Logs Num: {logsStringified()}</div>
				<LogViewer logsQuery={l} />
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

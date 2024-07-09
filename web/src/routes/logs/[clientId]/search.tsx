import { debounce } from "@solid-primitives/scheduled";
import { useParams, useSearchParams } from "@solidjs/router";
import { createQuery } from "@tanstack/solid-query";
import { For, Show, createEffect, createSignal } from "solid-js";
import { fetchLogs } from "~/lib/store/log-store";

export default () => {
	const clientId = useParams().clientId;
	const [searchParams, setSearchParams] = useSearchParams<{
		search?: string;
	}>();
	const [search, setSearch] = createSignal(searchParams.search ?? "");

	const trigger = debounce((value: string) => {
		console.log("trigger", value);
		setSearchParams({ search: value });
	}, 500);

	createEffect(() => {
		trigger(search());
	});

	const logs = createQuery(() => ({
		queryKey: ["logs", clientId, searchParams.search],
		queryFn: async () => {
			return await fetchLogs(clientId, searchParams.search);
		},
	}));

	const logsStringified = () => {
		return logs.data?.map((log) => JSON.stringify(log.content)) ?? [];
	};

	return (
		<div class="rounded-md border-white p-2">
			<p>Search term: {search()}</p>
			<div>
				<label for="search">Search</label>
				<input
					id="search"
					type="text"
					class="border-white p-1 rounded-md text-black"
					value={search()}
					onInput={(e) => setSearch(e.currentTarget.value)}
				/>
			</div>

			<div class="border-red p-2">
				<Show when={logs.isFetching}>
					<p class="animate-bounce">Loading...</p>
				</Show>
				<For each={logsStringified()}>{(log) => <div>{log}</div>}</For>
			</div>
		</div>
	);
};

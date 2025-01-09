import { debounce } from "@solid-primitives/scheduled";
import {
	type RouteDefinition,
	type RouteSectionProps,
	useParams,
	useSearchParams,
} from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { LogViewer } from "@/components/log-viewer";
import {
	getArrayValueOfSearchParam,
	useSelectedInstances,
} from "@/lib/hooks/use-selected-instances";
import { createLogQueryOptions } from "@/lib/store/query";
import { TextField, TextFieldRoot } from "@/components/ui/textfield";
import { createEffect, createSignal } from "solid-js";

export const route = {
	load: async ({ params, location }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(
			location.query.instances,
		);
		const search = location.query.search as string | undefined;

		return await queryClient.ensureInfiniteQueryData(
			createLogQueryOptions(() => ({
				clientId,
				selectedInstances,
				searchQuery: search,
			})),
		);
	},
} satisfies RouteDefinition;

export default (_props: RouteSectionProps) => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const selectedInstances = useSelectedInstances();
	const [search, setSearch] = createDebouncedSearch();

	return (
		<div class="flex flex-col justify-start gap-2">
			<TextFieldRoot>
				<TextField
					placeholder="Search..."
					onInput={(e) => {
						setSearch(e.currentTarget.value);
					}}
				/>
			</TextFieldRoot>
			<div class="flex-grow">
				<LogViewer
					selectedInstances={selectedInstances()}
					clientId={clientId}
					searchQuery={search()}
				/>
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

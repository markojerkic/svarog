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
import { SearchCommnad } from "@/components/log-search";

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
	const [searchParams, setSearchParams] = useSearchParams<{
		search?: string;
	}>();
	const searchQuery = () => searchParams.search ?? "";

	return (
		<div class="flex flex-col justify-start gap-2">
			<SearchCommnad
				search={searchQuery()}
				onInput={(search) => {
					console.log("search", search);
					setSearchParams({ search });
				}}
			/>
			<div class="flex-grow">
				<LogViewer
					selectedInstances={selectedInstances()}
					clientId={clientId}
					searchQuery={searchQuery()}
				/>
			</div>
		</div>
	);
};

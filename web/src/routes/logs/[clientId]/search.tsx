import {
	type RouteDefinition,
	type RouteSectionProps,
	useNavigate,
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
import { Button } from "@/components/ui/button";

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
			<SearchInfo clientId={clientId} selectedInstances={selectedInstances()} />
		</div>
	);
};

const SearchInfo = (props: {
	clientId: string;
	selectedInstances: string[];
}) => {
	const navigate = useNavigate();

	return (
		<div class="fixed bottom-0 left-0 w-full translate-y-0 transform animate-slide-in bg-blue-500 py-4 text-center text-white shadow-lg transition-transform">
			<p>
				You are on the search page, and live reload of data is turned off. To
				enable live reload, please clear the search.
			</p>
			<Button
				class="mt-2 rounded-md bg-white px-4 py-2 text-blue-500 hover:bg-gray-200"
				onClick={() => {
					const searchParams = new URLSearchParams();
					for (const instance of props.selectedInstances) {
						searchParams.append("instance", instance);
					}

					navigate(`/logs/${props.clientId}?${searchParams.toString()}`);
				}}
			>
				Dismiss
			</Button>
		</div>
	);
};

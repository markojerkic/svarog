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
import { SearchCommnad } from "@/components/log-search";
import { Button } from "@/components/ui/button";
import { preloadLogStore } from "@/lib/hooks/use-log-store";

export const route = {
	load: async ({ params, location }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(
			location.query.instances,
		);
		const search = location.query.search as string | undefined;

		return await preloadLogStore(
			{ clientId, selectedInstances, searchQuery: search },
			queryClient,
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
	const navigateBack = useNavigateBack(() => ({
		clientId: clientId,
		selectedInstances: selectedInstances(),
	}));

	return (
		<div class="flex flex-col justify-start gap-2">
			<SearchCommnad
				search={searchQuery()}
				onInput={(search) => {
					if (search === "") {
						navigateBack();
						return;
					}
					setSearchParams({ search });
				}}
			/>
			<SearchInfo clientId={clientId} selectedInstances={selectedInstances()} />
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

const SearchInfo = (props: {
	clientId: string;
	selectedInstances: string[];
}) => {
	const navigateBack = useNavigateBack(() => ({
		clientId: props.clientId,
		selectedInstances: props.selectedInstances,
	}));
	let elementRef: HTMLDivElement | undefined;

	return (
		<div
			ref={elementRef}
			class="flex w-full items-center justify-center gap-2 bg-card py-2 text-center text-primary shadow-lg"
		>
			<span>
				You are on the search page, and live reload of data is turned off. To
				enable live reload, please clear the search.
			</span>
			<Button onClick={navigateBack}>Clear</Button>
		</div>
	);
};

const useNavigateBack = (
	props: () => {
		clientId: string;
		selectedInstances: string[];
	},
) => {
	const navigate = useNavigate();
	return () => {
		const searchParams = new URLSearchParams();
		for (const instance of props().selectedInstances) {
			searchParams.append("instance", instance);
		}
		navigate(`/logs/${props().clientId}?${searchParams.toString()}`);
	};
};

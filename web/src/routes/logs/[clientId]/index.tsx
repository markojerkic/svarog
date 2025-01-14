import {
	type RouteDefinition,
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
import { createLogSubscription } from "@/lib/store/connection";
import { SearchCommnad } from "@/components/log-search";
import { preloadLogStore } from "@/lib/hooks/use-log-store";
import { Instances } from "@/components/instances";
import { useInstancesOptions } from "@/lib/hooks/logs/use-instances";

export const route = {
	load: async ({ params, location }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(
			location.query.instances,
		);

		queryClient.prefetchQuery(useInstancesOptions(clientId));
		return await preloadLogStore({ clientId, selectedInstances }, queryClient);
	},
} satisfies RouteDefinition;

export default () => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const selectedInstances = useSelectedInstances();
	const [searchParams] = useSearchParams();
	const navigate = useNavigate();

	createLogSubscription(() => ({
		clientId: clientId,
		instances: selectedInstances(),
	}));

	return (
		<div class="flex flex-col justify-start gap-2">
			<div class="flex-grow">
				<Instances clientId={clientId} />
				<SearchCommnad
					search={(searchParams.search as string) ?? ""}
					onInput={(search) => {
						const params = new URLSearchParams();
						params.set("search", search);
						for (const instance of selectedInstances()) {
							params.append("instances", instance);
						}

						if (document.startViewTransition) {
							document.startViewTransition(() => {
								navigate(`search?${params.toString()}`, { replace: true });
							});
						} else {
							navigate(`search?${params.toString()}`, { replace: true });
						}
					}}
				/>
				<LogViewer
					selectedInstances={selectedInstances()}
					clientId={clientId}
					selectedLogLineId={searchParams.logLine as string}
				/>
			</div>
		</div>
	);
};

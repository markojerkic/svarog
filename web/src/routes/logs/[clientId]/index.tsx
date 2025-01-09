import {
	type RouteDefinition,
	type RouteSectionProps,
	useParams,
} from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { LogViewer } from "@/components/log-viewer";
import {
	getArrayValueOfSearchParam,
	useSelectedInstances,
} from "@/lib/hooks/use-selected-instances";
import { createLogQueryOptions } from "@/lib/store/query";
import { createLogSubscription } from "@/lib/store/connection";

export const route = {
	load: async ({ params, location }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(
			location.query.instances,
		);

		return await queryClient.ensureInfiniteQueryData(
			createLogQueryOptions(() => ({
				clientId,
				selectedInstances,
			})),
		);
	},
} satisfies RouteDefinition;

export default (_props: RouteSectionProps) => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const selectedInstances = useSelectedInstances();

	//const instances = createQuery(() => ({
	//	queryKey: ["logs", "instances", clientId],
	//	queryFn: ({ signal }) => getInstances(clientId, signal),
	//	refetchOnWindowFocus: true,
	//}));

	//const wsActions = createLogSubscription(
	//	clientId,
	//	(line) => logQuery.data.insert(line),
	//	scrollToBottom,
	//	() => selectedInstances(),
	//);

	//useWithPreviousValue(
	//	() => logQuery.queryDetails.isFetched,
	//	(prev, curr) => {
	//		if (prev === false && curr === true) {
	//			scrollToBottom();
	//		}
	//	},
	//);

	//onMount(() => {
	//	wsActions.setInstances(selectedInstances());
	//	scrollToBottom();
	//});

	//createEffect(
	//	on(selectedInstances, (instances) => {
	//		wsActions.setInstances(instances);
	//		scrollToBottom();
	//	}),
	//);

	createLogSubscription(() => ({
		clientId: clientId,
		instances: selectedInstances(),
	}));

	return (
		<div class="flex flex-col justify-start gap-2">
			<div class="flex-grow">
				<LogViewer
					selectedInstances={selectedInstances()}
					clientId={clientId}
				/>
			</div>
		</div>
	);
};

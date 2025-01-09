import {
	type RouteDefinition,
	type RouteSectionProps,
	useParams,
} from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { createLogViewer } from "@/components/log-viewer";
import {
	getArrayValueOfSearchParam,
	useSelectedInstances,
} from "@/lib/hooks/use-selected-instances";
import { getInstances } from "@/lib/store/query";
import { createLogQuery } from "@/lib/store/query";

export const route = {
	load: async ({ params, location }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(
			location.query.instances,
		);

		createLogQuery(
			() => clientId,
			() => selectedInstances,
			() => undefined,
		);
		await queryClient.prefetchQuery({
			queryKey: ["logs", "instances", clientId],
			queryFn: ({ signal }) => getInstances(clientId, signal),
		});
	},
} satisfies RouteDefinition;

export default (_props: RouteSectionProps) => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const selectedInstances = useSelectedInstances();

	const logQuery = createLogQuery(
		() => clientId,
		selectedInstances,
		() => undefined,
	);
	const logCount = () => logQuery.logs.length;

	//const instances = createQuery(() => ({
	//	queryKey: ["logs", "instances", clientId],
	//	queryFn: ({ signal }) => getInstances(clientId, signal),
	//	refetchOnWindowFocus: true,
	//}));

	const [LogViewer] = createLogViewer();

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

	return (
		<div class="flex flex-col justify-start gap-2">
			<div class="flex-grow">
				<pre>Local log count: {logCount()}</pre>
				<pre>Is fetched: {logQuery.query.isFetched ? "je" : "nije"}</pre>
				<LogViewer logsQuery={logQuery} />
			</div>
		</div>
	);
};

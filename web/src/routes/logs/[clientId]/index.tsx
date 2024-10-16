import {
	type RouteDefinition,
	useLocation,
	useParams,
	useSearchParams,
} from "@markojerkic/solid-router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import {
	ErrorBoundary,
	Show,
	Suspense,
	createEffect,
	createMemo,
	on,
	onMount,
} from "solid-js";
import { Instances } from "~/components/instances";
import { createLogViewer } from "~/components/log-viewer";
import { createLogSubscription } from "~/lib/store/connection";
import { getInstances } from "~/lib/store/log-store";
import { createLogQuery } from "~/lib/store/query";

const getArrayValueOfSearchParam = (
	searchParam: string | string[] | undefined,
) => {
	if (searchParam === undefined) {
		return [];
	}

	return Array.isArray(searchParam) ? searchParam : [searchParam];
};

export const route = {
	load: async ({ params }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		// const selectedInstances = getArrayValueOfSearchParam(
		// 	location.query.instances,
		// );

		queryClient.prefetchQuery({
			queryKey: ["logs", "instances", clientId],
			queryFn: ({ signal }) => getInstances(clientId, signal),
		});

		// const logData = createLogQuery(() => ({ clientId, selectedInstances }));
		//
		// return await logData.fetchPreviousPage();
	},
} satisfies RouteDefinition;

export default () => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const [searchParams] = useSearchParams();
	const selectedInstances = createMemo(
		on(
			() => useLocation().search,
			() => {
				return getArrayValueOfSearchParam(searchParams.instance);
			},
		),
	);
	const logs = createLogQuery(
		() => clientId,
		selectedInstances,
		() => undefined,
	);
	const logCount = () => logs.logCount;

	const instances = createQuery(() => ({
		queryKey: ["logs", "instances", clientId],
		queryFn: ({ signal }) => getInstances(clientId, signal),
		refetchOnWindowFocus: true,
	}));

	const [LogViewer, scrollToBottom] = createLogViewer();

	const wsActions = createLogSubscription(
		clientId,
		(line) => logs.data.insert(line),
		scrollToBottom,
		() => selectedInstances(),
	);

	onMount(() => {
		wsActions.setInstances(selectedInstances());
		dispatchEvent(new Event("scroll-to-bottom"));
	});

	createEffect(
		on(selectedInstances, (instances) => {
			wsActions.setInstances(instances);
			dispatchEvent(new Event("scroll-to-bottom"));
		}),
	);

	return (
		<div class="flex flex-col justify-start gap-2">
			<ErrorBoundary fallback={<span class="bg-red-900 p-2">Error </span>}>
				<Suspense fallback={<div>Loading...</div>}>
					<Show when={instances.data}>
						{(instances) => (
							<Instances instances={instances()} actions={wsActions} />
						)}
					</Show>
				</Suspense>
			</ErrorBoundary>
			<div class="flex-grow">
				<pre>Local log count: {logCount()}</pre>
				<LogViewer logsQuery={logs} />
			</div>
		</div>
	);
};

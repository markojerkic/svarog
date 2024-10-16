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
import { useWithPreviousValue } from "~/lib/hooks/with-previous-value";
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
	load: async ({ params, location }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(
			location.query.instances,
		);

		queryClient.prefetchQuery({
			queryKey: ["logs", "instances", clientId],
			queryFn: ({ signal }) => getInstances(clientId, signal),
		});

		createLogQuery(
			() => clientId,
			() => selectedInstances,
			() => undefined,
		);
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

	useWithPreviousValue(
		() => logs.queryDetails.isFetched,
		(prev, curr) => {
			if (prev === false && curr === true) {
				scrollToBottom();
			}
		},
	);

	onMount(() => {
		wsActions.setInstances(selectedInstances());
		scrollToBottom();
	});

	createEffect(
		on(selectedInstances, (instances) => {
			wsActions.setInstances(instances);
			scrollToBottom();
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
				<pre>Is fetched: {logs.queryDetails.isFetched ? "je" : "nije"}</pre>
				<LogViewer logsQuery={logs} />
			</div>
		</div>
	);
};

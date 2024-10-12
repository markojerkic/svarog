import { type RouteDefinition, useParams } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { ErrorBoundary, Show, Suspense } from "solid-js";
import { Instances } from "~/components/instances";
import { createLogViewer } from "~/components/log-viewer";
import { createLogSubscription } from "~/lib/store/connection";
import { createLogQuery, getInstances } from "~/lib/store/log-store";

export const route = {
	load: async ({ params }) => {
		const queryClient = useQueryClient();
		const clientId = params.clientId;

		queryClient.prefetchQuery({
			queryKey: ["logs", "instances", clientId],
			queryFn: ({ signal }) => getInstances(clientId, signal),
		});

		const logData = createLogQuery(() => ({ clientId }));

		return await logData.fetchPreviousPage();
	},
} satisfies RouteDefinition;

export default () => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const logs = createLogQuery(() => ({ clientId }));

	const instances = createQuery(() => ({
		queryKey: ["logs", "instances", clientId],
		queryFn: ({ signal }) => getInstances(clientId, signal),
		refetchOnWindowFocus: true,
	}));

	const [LogViewer, scrollToBottom] = createLogViewer();

	const wsActions = createLogSubscription(
		clientId,
		logs.state.logStore,
		scrollToBottom,
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
				<LogViewer logsQuery={logs} />
			</div>
		</div>
	);
};

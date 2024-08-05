import { type RouteDefinition, useParams } from "@solidjs/router";
import { Show } from "solid-js";
import { createLogViewer } from "~/components/log-viewer";
import { createLogSubscription } from "~/lib/store/connection";
import { createLogQuery } from "~/lib/store/log-store";

export const route = {
	load: async ({ params }) => {
		const clientId = params.clientId;

		const logData = createLogQuery(clientId);
		return await logData.fetchPreviousPage();
	},
} satisfies RouteDefinition;

export default () => {
	const clientId = useParams<{ clientId: string }>().clientId;
	const logs = createLogQuery(clientId);
	const logCount = () => logs.logStore.size;

	const [LogViewer, scrollToBottom] = createLogViewer();

	createLogSubscription(clientId, logs.logStore, scrollToBottom);

	// createEffect(() => {
	// 	const newLogCount = logCount();
	// 	if (isLockedInBottom()) {
	// 		virtualizer.scrollToIndex(newLogCount, { align: "end" });
	// 	}
	// });

	return (
		<div>
			<button
				class="bg-green-500 p-1 rounded-md text-white"
				onClick={() => logs.fetchPreviousPage()}
				type="button"
				disabled={logs.isPreviousPageLoading()}
			>
				<Show
					when={logs.isPreviousPageLoading()}
					fallback={"Fetch previous page"}
				>
					<span class="animate-bounce">...</span>
				</Show>
			</button>

			<pre>Total: {logCount()}</pre>

			<LogViewer logsQuery={logs} />

			<button
				class="bg-blue-500 p-1 rounded-md text-white"
				onClick={() => logs.fetchNextPage()}
				type="button"
				disabled={logs.isNextPageLoading()}
			>
				Fetch next
			</button>
		</div>
	);
};

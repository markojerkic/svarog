import { Instances } from "@/components/logs/instances";
import { SearchCommnad } from "@/components/logs/log-search";
import { LogViewer } from "@/components/logs/log-viewer";
import { fetchLogPage } from "@/lib/hooks/use-log-store";
import { getArrayValueOfSearchParam } from "@/lib/hooks/use-selected-instances";
import { createLogSubscription } from "@/lib/store/connection";
import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import * as v from "valibot";

const searchParamsSchema = v.object({
	instances: v.optional(v.array(v.string()), []),
	logLine: v.optional(v.string()),
	search: v.optional(v.string()),
});

export const Route = createFileRoute("/__authenticated/logs/$clientId/")({
	component: RouteComponent,
	validateSearch: searchParamsSchema,
	loaderDeps: ({ search }) => ({
		instances: search.instances ?? [],
		logLine: search.logLine ?? "",
		search: search.search ?? "",
	}),
	staleTime: 2_000,
	loader: async ({ params, deps, abortController }) => {
		const clientId = params.clientId;
		const selectedInstances = getArrayValueOfSearchParam(deps.instances);
		const logLine = deps.logLine;

		console.log("loader");

		return fetchLogPage(
			clientId,
			{ logLine, selectedInstances, cursor: null },
			abortController.signal,
		);
	},
});

function RouteComponent() {
	const params = Route.useParams();
	const searchParams = Route.useSearch();
	const clientId = () => params().clientId;
	const selectedInstances = () => searchParams().instances;
	const search = () => searchParams().search;
	const navigate = useNavigate();

	createLogSubscription(() => ({
		clientId: clientId(),
		instances: selectedInstances(),
	}));

	return (
		<div class="flex flex-col justify-start gap-2 pb-4">
			<div class="sticky top-0 flex items-center gap-2 bg-white px-4 py-1">
				<Instances clientId={clientId()} />
				<SearchCommnad
					search={search()}
					onInput={(search) => {
						const params = new URLSearchParams();
						params.set("search", search);
						for (const instance of selectedInstances()) {
							params.append("instance", instance);
						}

						if (document.startViewTransition) {
							document.startViewTransition(() => {
								navigate({ to: `search?${params.toString()}`, replace: true });
							});
						} else {
							navigate({ to: `search?${params.toString()}`, replace: true });
						}
					}}
				/>
			</div>
			<LogViewer
				selectedInstances={selectedInstances()}
				clientId={clientId()}
				selectedLogLineId={searchParams().logLine}
			/>
		</div>
	);
}

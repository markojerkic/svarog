import { Instances } from "@/components/instances";
import { SearchCommnad } from "@/components/log-search";
import { LogViewer } from "@/components/log-viewer";
import { fetchLogPage } from "@/lib/hooks/use-log-store";
import { getArrayValueOfSearchParam } from "@/lib/hooks/use-selected-instances";
import { createLogSubscription } from "@/lib/store/connection";
import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import * as v from "valibot";

const searchParamsSchema = v.object({
	instances: v.optional(v.array(v.string()), []),
	//instances: v.pipe(
	//	v.optional(v.string(), "[]"),
	//	v.transform((value) => JSON.parse(value)),
	//	v.array(v.string()),
	//),
	logLine: v.optional(v.string()),
	search: v.optional(v.string()),
});

export const Route = createFileRoute("/logs/$clientId/")({
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
		<div class="flex flex-col justify-start gap-2">
			<div class="grow">
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
				<LogViewer
					selectedInstances={selectedInstances()}
					clientId={clientId()}
					selectedLogLineId={searchParams().logLine}
				/>
			</div>
		</div>
	);
}

import { ServiceListItem } from "@/components/service-card";
import type { Client } from "@/lib/hooks/logs/use-clients";
import { api } from "@/lib/utils/axios-api";
import { createFileRoute } from "@tanstack/solid-router";
import { For } from "solid-js";

export const Route = createFileRoute("/__authenticated/")({
	component: Home,
	loader: async ({ abortController }) => {
		return api
			.get<Client[]>("/v1/logs/clients", { signal: abortController.signal })
			.then((response) => response.data);
	},
});

export default function Home() {
	const clients = Route.useLoaderData();

	return (
		<main class="mx-auto p-4 text-center text-gray-700">
			<For
				each={clients()}
				fallback={<div class="animate-bounce text-white">Loading...</div>}
			>
				{(client) => <ServiceListItem clientId={client.clientId} />}
			</For>
		</main>
	);
}

import { ServiceListItem } from "@/components/service-card";
import { useClients, useClientsOptions } from "@/lib/hooks/logs/use-clients";
import type { RouteDefinition } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { For } from "solid-js";

export const route = {
	preload: async () => {
		const queryClient = useQueryClient();

		return queryClient.prefetchQuery(useClientsOptions());
	},
} satisfies RouteDefinition;

export default function Home() {
	const clients = useClients();

	return (
		<main class="mx-auto p-4 text-center text-gray-700">
			<For
				each={clients.data}
				fallback={<div class="animate-bounce text-white">Loading...</div>}
			>
				{(client) => <ServiceListItem clientId={client.clientId} />}
			</For>
		</main>
	);
}

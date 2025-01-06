import { ServiceListItem } from "@/components/service-card";
import { api } from "@/lib/utils/axios-api";
import type { RouteDefinition } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { For } from "solid-js";

const getClients = async () => {
	const response =
		await api.get<{ Client: { clientId: string } }[]>("/v1/logs/clients");
	return response.data;
};

export const route = {
	preload: async () => {
		return await useQueryClient().prefetchQuery({
			queryKey: ["clients"],
			queryFn: () => getClients(),
		});
	},
} satisfies RouteDefinition;

export default function Home() {
	const clients = createQuery(() => ({
		queryKey: ["clients"],
		queryFn: () => getClients(),
	}));

	return (
		<main class="mx-auto p-4 text-center text-gray-700">
			<For
				each={clients.data}
				fallback={<div class="animate-bounce text-white">Loading...</div>}
			>
				{(client) => <ServiceListItem clientId={client.Client.clientId} />}
			</For>
		</main>
	);
}

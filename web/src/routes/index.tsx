import { A, type RouteDefinition } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { For } from "solid-js";

const getClients = async () => {
	const response = await fetch(`${import.meta.env.VITE_API_URL}/v1/clients`);
	return response.json() as Promise<{ Client: { clientId: number } }[]>;
};

export const route = {
	load: async () => {
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
		<main class="text-center mx-auto text-gray-700 p-4">
			<For
				each={clients.data}
				fallback={<div class="text-white animate-bounce">Loading...</div>}
			>
				{(client) => (
					<div>
						<A
							href={`/logs/${client.Client.clientId}`}
							class="text-blue-500 hover:underline"
						>
							Client {client.Client.clientId}
						</A>
					</div>
				)}
			</For>
		</main>
	);
}

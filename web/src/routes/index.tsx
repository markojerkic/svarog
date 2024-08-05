import { A, type RouteDefinition, cache, createAsync } from "@solidjs/router";
import { For, Suspense } from "solid-js";

const getClients = cache(async () => {
	const response = await fetch(`${import.meta.env.VITE_API_URL}/v1/clients`);
	return response.json() as Promise<{ Client: { clientId: number } }[]>;
}, "clients");

export const route = {
	load: () => getClients(),
} satisfies RouteDefinition;

export default function Home() {
	const clients = createAsync(() => getClients());

	return (
		<main class="text-center mx-auto text-gray-700 p-4">
			<Suspense
				fallback={<div class="text-white animate-bounce">Loading...</div>}
			>
				<For each={clients()}>
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
			</Suspense>
		</main>
	);
}

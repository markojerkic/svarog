import { api } from "@/lib/utils/axios-api";
import { createQuery, type QueryOptions } from "@tanstack/solid-query";

export type Client = {
	clientId: string;
};

export const useClientsOptions = () =>
	({
		queryKey: ["clients"],
		queryFn: async ({ signal }) => {
			return api
				.get<Client[]>("/v1/logs/clients", { signal })
				.then((response) => response.data);
		},
	}) satisfies QueryOptions;

export const useClients = () => {
	return createQuery(() => useClientsOptions());
};

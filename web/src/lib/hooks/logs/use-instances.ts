import { api } from "@/lib/utils/axios-api";
import { createQuery, type QueryOptions } from "@tanstack/solid-query";

export const useInstancesOptions = (clientId: string) =>
	({
		queryKey: ["instances"],
		queryFn: async ({ signal }) => {
			return api
				.get<string[]>(`/v1/logs/${clientId}/instances`, { signal })
				.then((response) => response.data);
		},
	}) satisfies QueryOptions;

export const useInstances = (clientId: () => string) => {
	return createQuery(() => useInstancesOptions(clientId()));
};

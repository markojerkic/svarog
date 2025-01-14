import { api } from "@/lib/utils/axios-api";
import { createQuery, type QueryOptions } from "@tanstack/solid-query";

export type Instance = {
	clientId: string;
	ipAddress: string;
};

export const useInstancesOptions = () =>
	({
		queryKey: ["instances"],
		queryFn: async ({ signal }) => {
			return api
				.get<Instance[]>("/v1/logs/clients", { signal })
				.then((response) => response.data);
		},
	}) satisfies QueryOptions;

export const useInstances = () => {
	return createQuery(() => ({
		...useInstancesOptions(),
	}));
};

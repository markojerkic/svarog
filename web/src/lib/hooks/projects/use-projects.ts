import { api } from "@/lib/utils/axios-api";
import { useQuery, type QueryOptions } from "@tanstack/solid-query";

export type Project = {
	id: string;
	name: string;
	clients: string[];
	totalStorageSize: number;
};

export const useProjectsOptions = () =>
	({
		queryKey: ["projects"],
		queryFn: async ({ signal }) => {
			return api
				.get<Project[]>("/v1/projects", { signal })
				.then((res) => res.data);
		},
	}) satisfies QueryOptions;

export const useProjects = () => {
	return useQuery(useProjectsOptions);
};

import { api } from "@/lib/utils/axios-api";
import { createQuery, type QueryOptions } from "@tanstack/solid-query";

export type Project = {
	id: string;
	name: string;
	clients: string[];
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
	return createQuery(useProjectsOptions);
};

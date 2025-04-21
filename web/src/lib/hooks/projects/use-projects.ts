import { api } from "@/lib/utils/axios-api";

export type Project = {
	id: string;
	name: string;
	clients: string[];
	totalStorageSize: number;
};

export const getProjects = (signal: AbortSignal) =>
	api.get<Project[]>("/v1/projects", { signal }).then((res) => res.data);

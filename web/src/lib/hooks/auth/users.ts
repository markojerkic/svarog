import { api } from "@/lib/utils/axios-api";
import { createQuery, type QueryOptions } from "@tanstack/solid-query";

export type User = {
	username: string;
	firstName: string;
	lastName: string;
	id: string;
	role: string;
};
export const useUsers = (page: () => number) => {
	return createQuery(() => ({
		...useUsers.QUERY_OPTIONS(page()),
		enabled: !Number.isNaN(page()),
	}));
};
useUsers.QUERY_KEY = (page: number) => ["users", `users-${page}`];
useUsers.QUERY_OPTIONS = (page: number) =>
	({
		queryKey: useUsers.QUERY_KEY(page),
		queryFn: async () => {
			const response = await api.get<User[]>("/v1/auth/users", {
				params: {
					page: page,
				},
			});
			return response.data;
		},
	}) satisfies QueryOptions;

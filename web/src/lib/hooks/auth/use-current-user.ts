import { useQuery, type QueryOptions } from "@tanstack/solid-query";
import { api } from "@/lib/utils/axios-api";

export type LoggedInUser = {
	id: string;
	username: string;
	role: string;
};

export const useCurrentUser = () => {
	const user = useQuery(() => currentUserQueryOptions);

	return user;
};
useCurrentUser.QUERY_KEY = "current-user";

export const currentUserQueryOptions = {
	queryKey: [useCurrentUser.QUERY_KEY],
	queryFn: async () => {
		const response = await api.get<LoggedInUser>("/v1/auth/current-user");
		return response.data;
	},
} satisfies QueryOptions;

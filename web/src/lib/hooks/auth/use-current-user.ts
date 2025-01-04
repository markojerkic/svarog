import { createQuery } from "@tanstack/solid-query";
import { api } from "@/lib/utils/axios-api";

type LoggedInUser = {
	id: string;
	username: string;
	role: string;
};
export const useCurrentUser = () => {
	const user = createQuery(() => ({
		queryKey: [useCurrentUser.QUERY_KEY],
		queryFn: async () => {
			return api.get<LoggedInUser>("/v1/auth/current-user");
		},
	}));

	return user;
};

useCurrentUser.QUERY_KEY = "current-user";

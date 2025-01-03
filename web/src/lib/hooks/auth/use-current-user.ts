import { createQuery } from "@tanstack/solid-query";
import { NotLoggedInError } from "@/lib/errors/not-logged-in-error";

type LoggedInUser = {
	id: string;
	username: string;
	role: string;
};
export const useCurrentUser = () => {
	const user = createQuery(() => ({
		queryKey: [useCurrentUser.QUERY_KEY],
		queryFn: async () => {
			const response = await fetch(
				`${import.meta.env.VITE_API_URL}/v1/auth/current-user`,
			);
			if (!response.ok) {
				throw new NotLoggedInError();
			}

			return response.json() as Promise<LoggedInUser>;
		},
	}));

	return user;
};

useCurrentUser.QUERY_KEY = "current-user";

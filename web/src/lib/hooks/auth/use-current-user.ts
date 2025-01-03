import { createQuery } from "@tanstack/solid-query";

export const useCurrentUser = () => {
	const user = createQuery(() => ({
		queryKey: [useCurrentUser.QUERY_KEY],
		queryFn: async () => {
			const response = await fetch(`${import.meta.env.VITE_API_URL}/v1/user`);
			return response.json();
		},
	}));
};

useCurrentUser.QUERY_KEY = "current-user";

import { createMutation, useQueryClient } from "@tanstack/solid-query";
import * as v from "valibot";
import { useCurrentUser } from "./use-current-user";

export const loginSchema = v.object({
	email: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your email"),
		v.email("Please enter a valid email"),
	),
	password: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your password"),
		v.minLength(6, "Password must be at least 6 characters"),
	),
});
export type LoginInput = v.InferInput<typeof loginSchema>;

export const useLogin = () => {
	const queryClient = useQueryClient();

	return createMutation(() => ({
		mutationKey: ["login"],
		mutationFn: async (input: LoginInput) => {
			const response = await fetch(
				`${import.meta.env.VITE_API_URL}/v1/auth/login`,
				{
					method: "POST",
					headers: {
						"Content-Type": "application/json",
					},
					body: JSON.stringify(input),
				},
			);
			return response.json();
		},
		onSuccess: () => {
			queryClient.invalidateQueries({
				queryKey: [useCurrentUser.QUERY_KEY],
			});
		},
	}));
};

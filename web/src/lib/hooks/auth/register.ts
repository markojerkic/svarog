import { ApiError, type TApiError } from "@/lib/errors/api-error";
import type { FormStore } from "@modular-forms/solid";
import { createMutation, useQueryClient } from "@tanstack/solid-query";
import * as v from "valibot";
import { useCurrentUser } from "./use-current-user";
import { api } from "@/lib/utils/axios-api";

export const registerSchema = v.object({
	username: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your email"),
		v.minLength(5, "Username must be at least 5 characters"),
	),
	firstName: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter users's first name"),
		v.minLength(3, "First name must be at least 3 characters"),
	),
	lastName: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter users's last name"),
		v.minLength(3, "Last name must be at least 3 characters"),
	),

	password: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your password"),
		v.minLength(6, "Password must be at least 6 characters"),
	),
	repeatedPassword: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your password"),
		v.minLength(6, "Password must be at least 6 characters"),
	),
});
export type RegisterInput = v.InferInput<typeof registerSchema>;

export const useRegister = (form: FormStore<RegisterInput>) => {
	const queryClient = useQueryClient();

	return createMutation(() => ({
		mutationKey: ["register"],
		mutationFn: async (input: RegisterInput) => {
			return api.post<void, TApiError>("/v1/auth/register", {
				...input,
				repeatedPassword: undefined,
			});
		},
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: [useCurrentUser.QUERY_KEY],
			});
		},
		onError: (error) => {
			if (error instanceof ApiError) {
				error.setFormFieldErrors(form);
			}
		},
	}));
};

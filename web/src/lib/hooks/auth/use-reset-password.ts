import { ApiError, type TApiError } from "@/lib/errors/api-error";
import type { FormStore } from "@modular-forms/solid";
import { createMutation, useQueryClient } from "@tanstack/solid-query";
import * as v from "valibot";
import { useCurrentUser } from "./use-current-user";
import { api } from "@/lib/utils/axios-api";

export const resetPasswordSchema = v.object({
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
export type ResetPasswordInput = v.InferInput<typeof resetPasswordSchema>;

export const useResetPassword = (form: FormStore<ResetPasswordInput>) => {
	const queryClient = useQueryClient();

	return createMutation(() => ({
		mutationKey: ["reset-password"],
		mutationFn: async (input: ResetPasswordInput) => {
			return api.post<void, TApiError>("/v1/auth/reset-password", input);
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

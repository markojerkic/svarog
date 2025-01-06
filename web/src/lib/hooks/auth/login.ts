import { ApiError, type TApiError } from "@/lib/errors/api-error";
import type { FormStore } from "@modular-forms/solid";
import { createMutation, useQueryClient } from "@tanstack/solid-query";
import * as v from "valibot";
import { useCurrentUser } from "./use-current-user";
import { api } from "@/lib/utils/axios-api";
import { useNavigate } from "@solidjs/router";

export const loginSchema = v.object({
	username: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your email"),
		v.minLength(5, "Username must be at least 5 characters"),
	),
	password: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your password"),
		v.minLength(6, "Password must be at least 6 characters"),
	),
});
export type LoginInput = v.InferInput<typeof loginSchema>;

export const useLogin = (form: FormStore<LoginInput>) => {
	const queryClient = useQueryClient();

	return createMutation(() => ({
		mutationKey: ["login"],
		mutationFn: async (input: LoginInput) => {
			return api.post<void, TApiError>("/v1/auth/login", input);
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

export const useLoginWithToken = () => {
	const queryClient = useQueryClient();

	return createMutation(() => ({
		mutationKey: ["login"],
		mutationFn: async (token: string) => {
			return api.post<void, TApiError>("/v1/auth/login/token", {
				token,
			});
		},
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: [useCurrentUser.QUERY_KEY],
			});
		},
	}));
};

export const useLogout = () => {
	const queryClient = useQueryClient();
	const navigate = useNavigate();

	return createMutation(() => ({
		mutationKey: ["logout"],
		mutationFn: async () => {
			return api.post<void>("/v1/auth/logout");
		},
		onSuccess: async () => {
			return queryClient
				.invalidateQueries({
					queryKey: [useCurrentUser.QUERY_KEY],
				})
				.then(() => {
					navigate("/auth/login", { replace: true });
				});
		},
	}));
};

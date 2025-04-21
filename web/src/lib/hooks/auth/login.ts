import { ApiError, type TApiError } from "@/lib/errors/api-error";
import type { FormStore } from "@modular-forms/solid";
import { useMutation, useQueryClient } from "@tanstack/solid-query";
import * as v from "valibot";
import { useCurrentUser } from "./use-current-user";
import { api } from "@/lib/utils/axios-api";
import { useNavigate, useRouter } from "@tanstack/solid-router";

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

	return useMutation(() => ({
		mutationKey: ["login"],
		mutationFn: async (input: LoginInput) => {
			return api.post<void, TApiError>("/v1/auth/login", input);
		},
		onSuccess: async () => {
			return await queryClient.invalidateQueries({
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
	const router = useRouter();
	const navigate = useNavigate();

	return useMutation(() => ({
		mutationKey: ["login"],
		mutationFn: async (token: string) => {
			return api.post<void, TApiError>("/v1/auth/login/token", {
				token,
			});
		},
		onSuccess: async () => {
			return await router.invalidate();
		},
		onError: (error) => {
			if (
				error instanceof ApiError &&
				error.status === 401 &&
				error.message === "password_reset_required"
			) {
				navigate({ to: "/auth/reset-password" });
			}
		},
	}));
};

export const useLogout = () => {
	const router = useRouter();
	const navigate = useNavigate();

	return useMutation(() => ({
		mutationKey: ["logout"],
		mutationFn: async () => {
			return api.post<void>("/v1/auth/logout");
		},
		onSuccess: async () => {
			console.warn("on logout success");
			await router.invalidate();
			navigate({ to: "/auth/login" });
		},
	}));
};

import { ApiError, type TApiError } from "@/lib/api-error";
import type { FormStore } from "@modular-forms/solid";
import { createMutation, useQueryClient } from "@tanstack/solid-query";
import axios from "axios";
import * as v from "valibot";

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

export const api = axios.create({
	baseURL: import.meta.env.VITE_API_URL,
});
api.interceptors.response.use(
	(response) => response.data,
	(error) => {
		if (axios.isAxiosError(error)) {
			const apiError = error.response?.data;
			if (apiError) {
				throw new ApiError(apiError);
			}
		}

		throw error;
	},
);

export const useLogin = (_form: FormStore<LoginInput>) => {
	const _queryClient = useQueryClient();

	return createMutation(() => ({
		mutationKey: ["login"],
		mutationFn: async (input: LoginInput) => {
			return api.post<void, TApiError>("/v1/auth/login", input);
		},
	}));
};

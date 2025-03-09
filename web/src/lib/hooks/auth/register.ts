import { ApiError } from "@/lib/errors/api-error";
import type { FormStore } from "@modular-forms/solid";
import { createMutation } from "@tanstack/solid-query";
import * as v from "valibot";
import { api } from "@/lib/utils/axios-api";
import type { AxiosResponse } from "axios";
import { useRouter } from "@tanstack/solid-router";

export const registerSchema = v.object({
	username: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your email"),
		v.regex(/^[a-zA-Z0-9]+$/, "Username must not contain whitespaces"),
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
});
export type RegisterInput = v.InferInput<typeof registerSchema>;

export const useRegister = (form: FormStore<RegisterInput>) => {
	const router = useRouter();

	return createMutation(() => ({
		mutationKey: ["register"],
		mutationFn: async (input: RegisterInput) => {
			const response = await api.post<
				unknown,
				AxiosResponse<{ loginToken: string }>
			>("/v1/auth/register", {
				...input,
				repeatedPassword: undefined,
			});
			return response.data.loginToken;
		},
		onSuccess: () => {
			router.invalidate();
		},
		onError: (error) => {
			if (error instanceof ApiError) {
				error.setFormFieldErrors(form);
			}
		},
	}));
};

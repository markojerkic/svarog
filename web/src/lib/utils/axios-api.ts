import axios, { AxiosError } from "axios";
import { ApiError } from "@/lib/errors/api-error";
import { Router, useRouter } from "@tanstack/solid-router";
import { router } from "@/main";

export const api = axios.create({
	baseURL: import.meta.env.VITE_API_URL,
	withCredentials: true,
	paramsSerializer: (params) => {
		const searchParams = new URLSearchParams();
		for (const key in params) {
			if (!params[key]) {
				continue;
			}
			const value = params[key];

			console.log("Adding key: ", key, " with value: ", params[key]);
			if (Array.isArray(value)) {
				for (const val of value) {
					searchParams.append(key, val);
				}
			} else {
				searchParams.append(key, params[key]);
			}
		}
		return searchParams.toString();
	},
});
api.interceptors.response.use(
	(response) => response,
	(error) => {
		if (axios.isAxiosError(error)) {
			const apiError = error.response?.data;

			if (
				error.status === 401 &&
				apiError?.message === "password_reset_required"
			) {
				router.navigate({ to: "/auth/reset-password" });
			}

			if (apiError) {
				throw new ApiError(apiError, error.response?.status ?? 500);
			}
		} else if (
			error instanceof AxiosError &&
			error.message === "Network Error"
		) {
			throw new ApiError({ message: "Network error", fields: {} }, 0);
		}

		throw error;
	},
);

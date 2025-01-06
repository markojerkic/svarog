import axios, { AxiosError } from "axios";
import { ApiError } from "@/lib/errors/api-error";

export const api = axios.create({
	baseURL: import.meta.env.VITE_API_URL,
	withCredentials: true,
});
api.interceptors.response.use(
	(response) => response,
	(error) => {
		if (axios.isAxiosError(error)) {
			const apiError = error.response?.data;
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

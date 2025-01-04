import axios from "axios";
import { ApiError } from "@/lib/api-error";

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

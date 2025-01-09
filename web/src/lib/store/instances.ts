import { api } from "../utils/axios-api";

export const getInstances = async (
	clientId: string,
	abortSignal?: AbortSignal,
) => {
	return api
		.get<string[]>(`/v1/logs/${clientId}/instances`, {
			signal: abortSignal,
		})
		.then((response) => response.data);
};

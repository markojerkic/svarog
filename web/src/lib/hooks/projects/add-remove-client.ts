import { ApiError } from "@/lib/errors/api-error";
import { api } from "@/lib/utils/axios-api";
import type { FormStore } from "@modular-forms/solid";
import { createMutation, useQueryClient } from "@tanstack/solid-query";
import * as v from "valibot";

export const addClientSchema = v.object({
	projectId: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter project id"),
		v.minLength(3, "Project id must be at least 3 characters"),
	),
	clientName: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter client name"),
		v.minLength(3, "Client name must be at least 3 characters"),
	),
});

export const removeClientSchema = v.object({
	projectId: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter project id"),
		v.minLength(3, "Project id must be at least 3 characters"),
	),
	clientId: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter client name"),
		v.minLength(3, "Client name must be at least 3 characters"),
	),
});
export type AddClientInput = v.InferInput<typeof addClientSchema>;
export type RemoveClientInput = v.InferInput<typeof removeClientSchema>;

export const useAddClient = (form: FormStore<AddClientInput>) => {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationKey: ["add-client"],
		mutationFn: async (input: AddClientInput) => {
			const response = await api.post("/v1/projects/add-client", input);
			return response.data;
		},
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: ["projects"],
			});
		},
		onError: (error) => {
			if (error instanceof ApiError) {
				error.setFormFieldErrors(form);
			}
		},
	}));
};

export const useRemoveClient = (form: FormStore<RemoveClientInput>) => {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationKey: ["remove-client"],
		mutationFn: async (input: RemoveClientInput) => {
			const response = await api.post("/v1/projects/remove-client", input);
			return response.data;
		},
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: ["projects"],
			});
		},
		onError: (error) => {
			if (error instanceof ApiError) {
				error.setFormFieldErrors(form);
			}
		},
	}));
};

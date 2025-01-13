import { createMutation, useQueryClient } from "@tanstack/solid-query";
import type { Project } from "./use-projects";
import { api } from "@/lib/utils/axios-api";
import * as v from "valibot";
import type { FormStore } from "@modular-forms/solid";
import { ApiError } from "@/lib/errors/api-error";
import { clientNameSchema } from "./add-remove-client";

export const newProjectSchema = v.object({
	name: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter project name"),
		v.minLength(3, "Project name must be at least 3 characters"),
	),
	clients: v.optional(v.pipe(v.array(clientNameSchema), v.minLength(0))),
});
export type NewProjectInput = v.InferInput<typeof newProjectSchema>;

export const useCreateProject = (form: FormStore<NewProjectInput>) => {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationKey: ["projects"],
		mutationFn: async (project: NewProjectInput) => {
			const response = await api.post<Project>("/v1/projects", project);
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

import { createMutation, useQueryClient } from "@tanstack/solid-query";
import { api } from "@/lib/utils/axios-api";
import { toast } from "solid-sonner";

export const useDeleteProject = () => {
	const queryClient = useQueryClient();
	return createMutation(() => ({
		mutationKey: ["delete-project"],
		mutationFn: async (projectId: string) => {
			const response = await api.delete(`/v1/projects/${projectId}`);
			return response.data;
		},
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: ["projects"],
			});
		},
		onError: () => {
			toast.error("Failed to delete project");
		},
	}));
};

import { useMutation, useQueryClient } from "@tanstack/solid-query";
import { api } from "@/lib/utils/axios-api";
import { toast } from "solid-sonner";
import { useRouter } from "@tanstack/solid-router";

export const useDeleteProject = () => {
	const router = useRouter();

	return useMutation(() => ({
		mutationKey: ["delete-project"],
		mutationFn: async (projectId: string) => {
			const response = await api.delete(`/v1/projects/${projectId}`);
			return response.data;
		},
		onSuccess: () => {
			router.invalidate();
		},
		onError: () => {
			toast.error("Failed to delete project");
		},
	}));
};

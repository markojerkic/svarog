import { NewProject } from "@/components/admin/projects/new-project";
import { ProjectList } from "@/components/admin/projects/project-list";
import {
	useProjects,
	useProjectsOptions,
} from "@/lib/hooks/projects/use-projects";
import { Title } from "@solidjs/meta";
import type { RouteDefinition } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { Suspense } from "solid-js";

export const projectsRoute = {
	preload: async () => {
		const queryClient = useQueryClient();

		return await queryClient.prefetchQuery(useProjectsOptions());
	},
} satisfies RouteDefinition;

export default () => {
	const projects = useProjects();

	return (
		<>
			<Title>Projects</Title>
			<Suspense fallback={<div>Loading...</div>}>
				<div class="grid grid-cols-1 p-4">
					<span class="flex justify-end">
						<NewProject />
					</span>
					<ProjectList projects={projects.data ?? []} />
				</div>
			</Suspense>
		</>
	);
};

import { NewProject } from "@/components/admin/projects/new-project";
import {
	useProjects,
	useProjectsOptions,
} from "@/lib/hooks/projects/use-projects";
import { Title } from "@solidjs/meta";
import type { RouteDefinition } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { For, Suspense } from "solid-js";

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
				<NewProject />
				<ul>
					<For each={projects.data}>{(project) => <li>{project.name}</li>}</For>
				</ul>
			</Suspense>
		</>
	);
};

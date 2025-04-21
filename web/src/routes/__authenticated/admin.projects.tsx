import { getProjects } from "@/lib/hooks/projects/use-projects";
import { createFileRoute } from "@tanstack/solid-router";
import { NewProject } from "@/components/admin/projects/new-project";
import { ProjectList } from "@/components/admin/projects/project-list";
import { Suspense } from "solid-js";

export const Route = createFileRoute("/__authenticated/admin/projects")({
	component: RouteComponent,
	loader: async ({ abortController }) => {
		return getProjects(abortController.signal);
	},
	head: () => ({
		meta: [{ title: "Projects" }],
	}),
});

function RouteComponent() {
	const projects = Route.useLoaderData();

	return (
		<>
			<Suspense fallback={<div>Loading...</div>}>
				<div class="grid grid-cols-1 p-4">
					<span class="flex justify-end">
						<NewProject />
					</span>
					<ProjectList projects={projects() ?? []} />
				</div>
			</Suspense>
		</>
	);
}

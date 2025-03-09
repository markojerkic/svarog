import { Nav } from "@/components/navigation/nav";
import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";

export const Route = createFileRoute("/__authenticated")({
	component: RouteComponent,
	beforeLoad: async ({ location, context }) => {
		if (context.authError && context.authError.status === 401) {
			console.log("context.authError", context.authError);
			const e = context.authError;
			if (e.message === "password_reset_required") {
				throw redirect({
					to: "/auth/reset-password",
					search: {
						redirect: location.pathname,
						redirectSearch: location.search,
					},
				});
			}
			throw redirect({
				to: "/auth/login",
				search: {
					redirect: location.pathname,
					redirectSearch: location.search,
				},
			});
		}
	},
});

function RouteComponent() {
	const context = Route.useRouteContext();
	console.warn("Layout pripaziti za log ekran");
	return (
		<>
			<Nav currentUser={context().auth!} />
			<Outlet />
		</>
	);
}

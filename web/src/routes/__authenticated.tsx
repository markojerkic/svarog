import { Nav } from "@/components/navigation/nav";
import {
	createFileRoute,
	Outlet,
	redirect,
	useMatch,
} from "@tanstack/solid-router";

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
	const match = useMatch({ from: "/__authenticated/logs/$clientId" });
	const isLogsRoute = () => match();

	console.warn("Layout pripaziti za log ekran", context().auth);
	return (
		<div
			class="flex flex-col justify-start"
			classList={{
				"h-screen": Boolean(isLogsRoute()),
			}}
		>
			<Nav currentUser={context().auth!} />
			<Outlet />
		</div>
	);
}

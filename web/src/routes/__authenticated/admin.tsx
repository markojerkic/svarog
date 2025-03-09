import { AdminLayout } from "@/components/admin/layout";
import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import { toast } from "solid-sonner";

export const Route = createFileRoute("/__authenticated/admin")({
	component: RouteComponent,
	beforeLoad: async ({ context }) => {
		if (context.auth?.role !== "admin") {
			toast.error("You are not an admin");
			throw redirect({
				to: "/",
			});
		}
	},
});

function RouteComponent() {
	return (
		<AdminLayout>
			<Outlet />
		</AdminLayout>
	);
}

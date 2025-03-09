import { AdminLayout } from "@/components/admin/layout";
import { router } from "@/main";
import { createFileRoute, Outlet } from "@tanstack/solid-router";
import { toast } from "solid-sonner";

export const Route = createFileRoute("/__authenticated/admin")({
	component: RouteComponent,
	beforeLoad: async ({ context }) => {
		if (context.auth?.role !== "admin") {
			router.history.push("/");
			toast.error("You are not an admin");
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

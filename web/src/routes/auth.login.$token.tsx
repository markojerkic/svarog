import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { useLoginWithToken } from "@/lib/hooks/auth/login";
import { createEffect } from "solid-js";
import { toast } from "solid-sonner";

export const Route = createFileRoute("/auth/login/$token")({
	component: RouteComponent,
});

function RouteComponent() {
	const parameters = Route.useParams();
	const navigate = useNavigate();
	const login = useLoginWithToken();

	createEffect(() => {
		const token = parameters().token;
		if (!token || token === "") {
			toast.error("Invalid login token");
			return;
		}

		login.mutate(token, {
			onError: () => {
				toast.error("Unable to login with given token");
			},
			onSuccess: () => {
				navigate({
					to: "/",
					replace: true,
				});
			},
		});
	});

	return <p>Logging in...</p>;
}

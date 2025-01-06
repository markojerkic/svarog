import { useCurrentUser } from "@/lib/hooks/auth/use-current-user";
import { useNavigate } from "@solidjs/router";
import { type ParentProps, Show, Suspense, createEffect } from "solid-js";

export default function ProtectedLayout(props: ParentProps) {
	const currentUser = useCurrentUser();
	const navigate = useNavigate();

	createEffect(() => {
		if (currentUser.isError && currentUser.error.status === 401) {
			if (currentUser.error.message === "password_reset_required") {
				navigate("/auth/reset-password");
				return;
			}

			navigate("/auth/login");
		}
	});

	return (
		<Suspense>
			<Show when={currentUser.isSuccess}>{props.children}</Show>
		</Suspense>
	);
}

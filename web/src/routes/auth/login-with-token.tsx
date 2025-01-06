import { useLoginWithToken } from "@/lib/hooks/auth/login";
import { useNavigate, useParams } from "@solidjs/router";
import { createEffect } from "solid-js";
import { toast } from "solid-sonner";

export default () => {
	const parameters = useParams<{ token: string }>();
	const navigate = useNavigate();
	const login = useLoginWithToken();

	createEffect(() => {
		const token = parameters.token;
		if (!token || token === "") {
			toast.error("Invalid login token");
			return;
		}

		login.mutate(token, {
			onError: () => {
				toast.error("Unable to login with given token");
			},
			onSuccess: () => {
				navigate("/", { replace: true });
			},
		});
	});

	return <p>Logging in...</p>;
};

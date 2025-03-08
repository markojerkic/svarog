import { useLogout } from "@/lib/hooks/auth/login";
import { onMount } from "solid-js";

export default () => {
	const logout = useLogout();

	onMount(() => {
		logout.mutate();
	});

	return <div>Logging out...</div>;
};

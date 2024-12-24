import { useMatch, } from "@solidjs/router";
import type { ParentProps } from "solid-js";
import { Nav } from "~/components/nav";

export const Layout = (props: ParentProps) => {
	const isLogsRoute = () =>
		useMatch(() => "/logs/:clientId") ||
		useMatch(() => "/logs/:clientId/search");

	return (
		<div
			class="flex flex-col justify-start"
			classList={{
				"h-screen": Boolean(isLogsRoute()),
			}}
		>
			<Nav />
			{props.children}
		</div>
	);
};

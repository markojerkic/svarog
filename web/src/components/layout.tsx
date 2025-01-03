import { useMatch, type RouteSectionProps } from "@solidjs/router";
import { Suspense } from "solid-js";
import { Nav } from "@/components/navigation/nav";

export const Layout = (props: RouteSectionProps<unknown>) => {
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
			<Suspense>
				<main class="p-4">{props.children}</main>
			</Suspense>
		</div>
	);
};

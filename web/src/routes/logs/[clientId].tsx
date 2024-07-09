import { A, useParams } from "@solidjs/router";
import type { VoidProps } from "solid-js";

export default (params: VoidProps) => {
	const clientId = useParams().clientId;

	return (
		<div>
			<p class="flex gap-2 p-2">
				<A href="search">Search</A>
			</p>
			<h1>Client ID: {clientId}</h1>
			{params.children}
		</div>
	);
};

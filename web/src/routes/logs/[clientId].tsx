import { A, useParams } from "@markojerkic/solid-router";
import type { VoidProps } from "solid-js";
import { Title } from "@solidjs/meta";

export default (params: VoidProps) => {
	const clientId = useParams().clientId;

	return (
		<div class="grow overflow-y-hidden">
			<p class="flex gap-2 p-2">
				<A href="search">Search</A>
			</p>
			<Title>Client: {clientId}</Title>
			<h1>Client: {clientId}</h1>
			{params.children}
		</div>
	);
};

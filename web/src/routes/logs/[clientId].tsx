import { A, type RouteSectionProps, useParams } from "@solidjs/router";
import { Title } from "@solidjs/meta";

export default (params: RouteSectionProps) => {
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

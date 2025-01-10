import { type RouteSectionProps, useParams } from "@solidjs/router";
import { Title } from "@solidjs/meta";

export default (params: RouteSectionProps) => {
	const clientId = useParams().clientId;

	return (
		<div class="grow overflow-y-hidden">
			<Title>Client: {clientId}</Title>
			{params.children}
		</div>
	);
};

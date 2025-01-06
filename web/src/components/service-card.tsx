import { A } from "@solidjs/router";
import { Card, CardDescription, CardHeader, CardTitle } from "./ui/card";

export const ServiceListItem = (props: { clientId: string }) => {
	return (
		<Card>
			<CardHeader>
				<CardTitle>
					<A href={`/logs/${props.clientId}`} class="hover:underline">
						{props.clientId}
					</A>
				</CardTitle>
				<CardDescription>Client ID</CardDescription>
			</CardHeader>
		</Card>
	);
};

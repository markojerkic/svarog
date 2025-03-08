import { Card, CardDescription, CardHeader, CardTitle } from "./ui/card";

export const ServiceListItem = (props: { clientId: string }) => {
	return (
		<Card>
			<CardHeader>
				<CardTitle>
					<a href={`/logs/${props.clientId}`} class="hover:underline">
						{props.clientId}
					</a>
				</CardTitle>
				<CardDescription>Client ID</CardDescription>
			</CardHeader>
		</Card>
	);
};

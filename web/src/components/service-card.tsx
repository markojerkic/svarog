import { Link } from "@tanstack/solid-router";
import { Card, CardDescription, CardHeader, CardTitle } from "./ui/card";

export const ServiceListItem = (props: { clientId: string }) => {
	return (
		<Card>
			<CardHeader>
				<CardTitle>
					<Link
						class="hover:underline"
						to="/logs/$clientId"
						params={() => ({ clientId: props.clientId })}
					>
						{props.clientId}
					</Link>
				</CardTitle>
				<CardDescription>Client ID</CardDescription>
			</CardHeader>
		</Card>
	);
};

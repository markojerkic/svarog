import { createLogSubscription } from "@/lib/store/connection";
import type { LogLine } from "@/lib/store/query";

export const LogSubscriber = (props: {
	clientId: string;
	selectedInstances: string[];
	searchQuery?: string;
	onNewLine: (line: LogLine) => void;
}) => {
	createLogSubscription(() => ({
		clientId: props.clientId,
		instances: props.selectedInstances,
		onNewLine: (line) => {
			console.log("New line", line);
			props.onNewLine(line);
		},
	}));

	return <></>;
};

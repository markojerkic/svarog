import { createReconnectingWS } from "@solid-primitives/websocket";
import { onCleanup, onMount } from "solid-js";
import type { SortedList } from "./sorted-list";
import type { LogLine } from "./log-store";

type MessageType =
	| "newLine"
	| "addSubscriptionInstance"
	| "removeSubscriptionInstance"
	| "ping"
	| "pong";

type WsMessage = {
	type: MessageType;
	data: unknown;
};

export const createLogSubscription = (
	clientId: string,
	logStore: SortedList<LogLine>,
) => {
	const ws = createReconnectingWS(`ws://localhost:1323/api/v1/ws/${clientId}`);

	onMount(() => {
		ws.addEventListener("message", (e) => {
			const message: WsMessage = JSON.parse(e.data);
			console.log("Message", message);
		});
	});

	onCleanup(() => {
		ws.close();
	});
};

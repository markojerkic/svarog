import { createReconnectingWS } from "@solid-primitives/websocket";
import { onCleanup, onMount } from "solid-js";
import type { SortedList } from "./sorted-list";
import type { LogLine } from "./log-store";

type MessageType =
	| "addSubscriptionInstance"
	| "removeSubscriptionInstance"
	| "ping"
	| "pong";

type WsMessage =
	| {
			type: "newLine";
			data: LogLine;
	  }
	| {
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
			try {
				const message: WsMessage = JSON.parse(e.data);
				if (message.type === "newLine") {
					console.log("Message", message.data.content);
                    logStore.insert(message.data);
				}
			} catch (e) {
				console.error("Error parsing WS message", e);
			}
		});
	});

	onCleanup(() => {
		ws.close();
	});
};

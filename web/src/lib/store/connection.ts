import { createReconnectingWS } from "@solid-primitives/websocket";
import { onCleanup, onMount } from "solid-js";
import type { SortedList } from "./sorted-list";
import type { LogLine } from "./log-store";

type MessageType = "addSubscriptionInstance" | "removeSubscriptionInstance";

type WsMessage =
	| {
			type: "setInstances";
			data: string[];
	  }
	| {
			type: "newLine";
			data: LogLine;
	  }
	| {
			type: "ping" | "pong";
			data?: undefined;
	  }
	| {
			type: MessageType;
			data: unknown;
	  };

export const createLogSubscription = (
	clientId: string,
	logStore: SortedList<LogLine>,
	scrollToBottom: () => void,
) => {
	const ws = createReconnectingWS(
		`${import.meta.env.VITE_WS_URL}/v1/ws/${clientId}`,
	);

	onMount(() => {
		ws.addEventListener("message", (e) => {
			try {
				const message: WsMessage = JSON.parse(e.data);
				if (message.type === "newLine") {
					logStore.insert(message.data);
					scrollToBottom();
				}
			} catch (e) {
				console.error("Error parsing WS message", e);
			}
		});
	});

	const pingPongInterval = setInterval(() => {
		ws.send(JSON.stringify({ type: "ping" } satisfies WsMessage));
	}, 10_000);

	onCleanup(() => {
		ws.close();
		clearInterval(pingPongInterval);
	});

	const setInstances = (instances: string[]) => {
		ws.send(
			JSON.stringify({
				type: "setInstances",
				data: instances,
			} satisfies WsMessage),
		);
	};

	const addSubscription = (instance: string) => {
		ws.send(
			JSON.stringify({
				type: "addSubscriptionInstance",
				data: instance,
			} satisfies WsMessage),
		);
	};
	const removeSubscription = (instance: string) => {
		ws.send(
			JSON.stringify({
				type: "removeSubscriptionInstance",
				data: instance,
			} satisfies WsMessage),
		);
	};

	return { addSubscription, removeSubscription, setInstances };
};

export type WsActions = ReturnType<typeof createLogSubscription>;

import { createReconnectingWS } from "@solid-primitives/websocket";
import { onCleanup, onMount } from "solid-js";
import type { LogLine } from "@/lib/hooks/use-log-store";
import { createEventBus } from "@solid-primitives/event-bus";

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

const newLogLineBus = createEventBus<LogLine[]>();

export const newLogLineListener = newLogLineBus.listen;
export const createLogSubscription = (
	props: () => {
		clientId: string;
		instances: string[];
	},
) => {
	const ws = createReconnectingWS(
		`${import.meta.env.VITE_WS_URL}/v1/ws/${props().clientId}`,
	);
	const buffer: LogLine[] = [];

	onMount(() => {
		ws.addEventListener("message", (e) => {
			try {
				const message: WsMessage = JSON.parse(e.data);
				if (message.type === "newLine") {
					buffer.push(message.data);
				}
			} catch (e) {
				console.error("Error parsing WS message", e);
			}
		});

		ws.addEventListener("open", () => {
			setInstances(props().instances);
		});

		const bufferDumpInterval = setInterval(() => {
			if (buffer.length > 0) {
				newLogLineBus.emit(buffer);
				buffer.length = 0;
			}
		}, 1000);

		return () => {
			clearInterval(bufferDumpInterval);
		};
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

	const close = () => {
		ws.close();
	};

	return { addSubscription, removeSubscription, setInstances, close };
};

export type WsActions = ReturnType<typeof createLogSubscription>;

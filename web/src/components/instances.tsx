import { For, createSignal } from "solid-js";
import { createStore } from "solid-js/store";
import type { WsActions } from "~/lib/store/connection";

export const instancesColorMap = createStore<{ [key: string]: string }>({});
export const useInstanceColor = (instance: string) => {
	const [state, setState] = instancesColorMap;
	if (!state[instance]) {
		const randomColor = randomColorForInstance(instance);
		setState(instance, randomColor);
	}
	return () => state[instance];
};

const Instance = (props: { instance: string; actions: WsActions }) => {
	const [isActive, setIsActive] = createSignal(true);
	const color = useInstanceColor(props.instance);

	const toggleInstance = () => {
		if (isActive()) {
			props.actions.removeSubscription(props.instance);
		} else {
			props.actions.removeSubscription(props.instance);
		}
		setIsActive(!isActive());
	};

	return (
		<button
			type="button"
			class="flex items-center gap-2 rounded-md border border-gray-900 p-1.5 text-black hover:bg-gray-200"
			classList={{
				"bg-gray-100": isActive(),
			}}
			onClick={toggleInstance}
		>
			<svg
				xmlns="http://www.w3.org/2000/svg"
				viewBox="0 0 100 100"
				class="h-4 w-4"
			>
				<circle cx="50" cy="50" r="25" fill={color()} />
			</svg>

			{props.instance}
		</button>
	);
};

export const Instances = (props: {
	instances: string[];
	actions: WsActions;
}) => {
	return (
		<nav class="gap-2 border border-sky-700 p-2">
			<p>Instances</p>
			<div class="scrollbar-thin scrollbar-track-white p-2 scrollbar-thumb-zinc-700 flex gap-2 overflow-x-scroll">
				<For each={props.instances}>
					{(instance) => (
						<Instance instance={instance} actions={props.actions} />
					)}
				</For>
			</div>
		</nav>
	);
};

const randomColorForInstance = (_: string) => {
	return `#${Math.floor(Math.random() * 16777215).toString(16)}`;
};

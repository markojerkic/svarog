import { For } from "solid-js";
import { createStore } from "solid-js/store";

export const instancesColorMap = createStore<{ [key: string]: string }>({});
export const useInstanceColor = (instance: string) => {
	const [state, setState] = instancesColorMap;
	if (!state[instance]) {
		const randomColor = randomColorForInstance(instance);
		setState(instance, randomColor);
	}
	return () => state[instance];
};

const Instance = (props: { instance: string }) => {
	const color = useInstanceColor(props.instance);
	const toggleInstance = () => {
		console.error("Not implemented: toggle instance");
	};

	return (
		<button
			type="button"
			class="flex items-center gap-2 rounded-3xl border border-sky-900 bg-sky-800 p-1.5"
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

export const Instances = (props: { instances: string[] }) => {
	return (
		<nav class="inline-flex bg-sky-700 p-2">
			<For each={props.instances}>
				{(instance) => <Instance instance={instance} />}
			</For>
		</nav>
	);
};

const randomColorForInstance = (_: string) => {
	return `#${Math.floor(Math.random() * 16777215).toString(16)}`;
};

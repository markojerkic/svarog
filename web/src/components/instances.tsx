import { useSearchParams } from "@markojerkic/solid-router";
import { For } from "solid-js";
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

const useIsInstanceActive = (instance: string) => {
	const [searchParams, setSearchParams] = useSearchParams<{
		instance: string[];
	}>();
	const isActive = () => {
		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
		return instanceSearchParams[instance] ?? false;
	};

	const mapInstances = (instances: string | string[]) => {
		const instancesMap: Record<string, boolean> = {};
		if (Array.isArray(instances)) {
			for (const instance of instances) {
				instancesMap[instance] = true;
			}
		} else {
			instancesMap[instances] = true;
		}
		return instancesMap;
	};

	const mergeInstancesMap = (
		instancesMap: Record<string, boolean>,
	): string[] => {
		return Object.keys(instancesMap).filter(
			(instance) => instancesMap[instance],
		);
	};

	const addInstance = () => {
		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
		instanceSearchParams[instance] = true;
		setSearchParams({ instance: mergeInstancesMap(instanceSearchParams) });
	};

	const removeInstance = () => {
		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
		instanceSearchParams[instance] = false;
		console.log(
			"Removing instance",
			instanceSearchParams,
			"merged",
			mergeInstancesMap(instanceSearchParams),
		);
		setSearchParams({ instance: mergeInstancesMap(instanceSearchParams) });
	};

	const toggleInstance = () => {
		console.log(
			isActive()
				? "Is active, so going to remove"
				: "Is not active, so going to add it",
		);
		if (isActive()) {
			removeInstance();
		} else {
			addInstance();
		}
	};

	return { isActive, addInstance, removeInstance, toggleInstance };
};

const Instance = (props: { instance: string; actions: WsActions }) => {
	const color = useInstanceColor(props.instance);
	const { isActive, toggleInstance } = useIsInstanceActive(props.instance);

	return (
		<button
			type="button"
			class="flex items-center gap-2 rounded-md border border-gray-900 p-1.5 text-black hover:bg-gray-200"
			classList={{
				"bg-gray-300": isActive(),
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
			<div class="scrollbar-thin scrollbar-track-white scrollbar-thumb-zinc-700 flex gap-2 overflow-x-scroll p-2">
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

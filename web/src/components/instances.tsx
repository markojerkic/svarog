import { useLocation, useSearchParams } from "@markojerkic/solid-router";
import { For, batch, createMemo, on } from "solid-js";
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

const AllInstances = () => {
	const { isActive, toggleInstance } = useInstanceIsActive("all");

	return (
		<button
			type="button"
			class="flex items-center gap-2 rounded-md border border-gray-900 p-1.5 text-black hover:bg-gray-200"
			classList={{
				"bg-gray-300": isActive(),
			}}
			onClick={toggleInstance}
		>
			All
		</button>
	);
};

const Instance = (props: { instance: string; actions: WsActions }) => {
	const color = useInstanceColor(props.instance);
	const { isActive, toggleInstance } = useInstanceIsActive(props.instance);

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

const mergeInstancesMap = (instancesMap: Record<string, boolean>): string[] => {
	return Object.keys(instancesMap).filter((instance) => instancesMap[instance]);
};

const useInstanceIsActive = (instance: string | "all") => {
	const [searchParams, setSearchParams] = useSearchParams<{
		all: string;
		instance: string | string[];
	}>();

	const allActive = createMemo(
		on(
			() => useLocation().search,
			() => {
				const instanceSelected =
					Object.keys(mapInstances(searchParams.instance ?? [])).length > 0;
				return (
					!instanceSelected &&
					(searchParams.all ? Boolean(searchParams.all) : true)
				);
			},
		),
	);

	const isActive = () => {
		if (instance === "all") {
			return allActive();
		}

		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
		return instanceSearchParams[instance] ?? false;
	};

	const addInstance = () => {
		if (instance === "all") {
			setSearchParams({ instance: undefined });
			return;
		}

		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
		instanceSearchParams[instance] = true;
		setSearchParams({
			instance: mergeInstancesMap(instanceSearchParams),
			// all: undefined,
		});
	};

	const removeInstance = () => {
		if (instance === "all") {
			console.warn("Cannot remove all instances");
			return;
		}

		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
		instanceSearchParams[instance] = false;
		const newInstances = mergeInstancesMap(instanceSearchParams);
		batch(() => {
			setSearchParams({ instance: newInstances });
			if (newInstances.length === 0) {
				// setSearchParams({ all: "true" });
			}
		});
	};

	const toggleInstance = () => {
		if (isActive()) {
			removeInstance();
		} else {
			addInstance();
		}
	};

	return { isActive, addInstance, removeInstance, toggleInstance };
};

// const useAllAreActive = () => {
// 	const [searchParams, setSearchParams] = useSearchParams<{
// 		all: string;
// 		instance: string | string[];
// 	}>();
// 	const allActive = createMemo(
// 		on(
// 			() => useLocation().search,
// 			() => {
// 				const instanceSelected =
// 					Object.keys(mapInstances(searchParams.instance ?? [])).length > 0;
// 				return (
// 					!instanceSelected &&
// 					(searchParams.all ? Boolean(searchParams.all) : true)
// 				);
// 			},
// 		),
// 	);
//
// 	createEffect(
// 		on(
// 			() => useLocation().search,
// 			() => {
// 				const instanceSelected =
// 					Object.keys(mapInstances(searchParams.instance ?? [])).length > 0;
// 				if (instanceSelected && searchParams.all) {
// 					console.log("Setting all to undefined");
// 					setSearchParams({ all: undefined });
// 				}
// 			},
// 		),
// 	);
//
// 	const setAllActive = (active: boolean) => {
// 		batch(() => {
// 			console.log("Setting all to", active);
// 			setSearchParams({ all: active ? "true" : undefined });
// 			if (!active) {
// 				setSearchParams({ instance: undefined });
// 			}
// 		});
// 	};
//
// 	return [allActive, setAllActive] as const;
// };

const mapInstances = (instances: string | string[]) => {
	const instancesMap: Record<string, boolean> = {};
	if (Array.isArray(instances)) {
		for (const instance of instances) {
			instancesMap[instance] = true;
		}
	} else if (instances !== undefined) {
		instancesMap[instances] = true;
	}
	return instancesMap;
};

// const useIsInstanceActive = (instance: string) => {
// 	const [searchParams, setSearchParams] = useSearchParams<{
// 		instance: string[];
// 	}>();
// 	const isActive = () => {
// 		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
//
// 		return instanceSearchParams[instance] ?? false;
// 	};
//
// 	const mergeInstancesMap = (
// 		instancesMap: Record<string, boolean>,
// 	): string[] => {
// 		return Object.keys(instancesMap).filter(
// 			(instance) => instancesMap[instance],
// 		);
// 	};
//
// 	const addInstance = () => {
// 		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
// 		instanceSearchParams[instance] = true;
// 		setSearchParams({ instance: mergeInstancesMap(instanceSearchParams) });
// 	};
//
// 	const removeInstance = () => {
// 		const instanceSearchParams = mapInstances(searchParams.instance ?? []);
// 		instanceSearchParams[instance] = false;
// 		setSearchParams({ instance: mergeInstancesMap(instanceSearchParams) });
// 	};
//
// 	const toggleInstance = () => {
// 		if (isActive()) {
// 			removeInstance();
// 		} else {
// 			addInstance();
// 		}
// 	};
//
// 	return { isActive, addInstance, removeInstance, toggleInstance };
// };

export const Instances = (props: {
	instances: string[];
	actions: WsActions;
}) => {
	return (
		<nav class="gap-2 border border-sky-700 p-2">
			<p>Instances</p>
			<div class="scrollbar-thin scrollbar-track-white scrollbar-thumb-zinc-700 flex gap-2 overflow-x-scroll p-2">
				<AllInstances />
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

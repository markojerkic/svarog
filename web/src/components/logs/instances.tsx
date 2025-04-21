import { For, Suspense, batch, createMemo, on } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import type { WsActions } from "@/lib/store/connection";
import { useInstances } from "@/lib/hooks/logs/use-instances";
import { Route } from "@/routes/__authenticated/logs.$clientId.index";

export const Instances = (props: { clientId: string }) => {
	const instances = useInstances(() => props.clientId);

	return (
		<nav class="border-sky-600 border-l-2 bg-gray-50 py-2">
			<Suspense
				fallback={<div class="px-3 py-2 text-gray-500 text-xs">Loading...</div>}
			>
				<div class="scrollbar-thin scrollbar-track-transparent scrollbar-thumb-gray-300 flex gap-1.5 overflow-x-auto px-3 py-2">
					<AllInstances />
					<For each={instances.data}>
						{(instance) => <Instance instance={instance} />}
					</For>
				</div>
			</Suspense>
		</nav>
	);
};

export const NOOP_WS_ACTIONS: WsActions = {
	setInstances: () => {},
	addSubscription: () => {},
	removeSubscription: () => {},
	close: () => {},
};

const AllInstances = () => {
	const { isActive, toggleInstance } = useInstanceIsActive("all");

	return (
		<button
			type="button"
			class="flex h-7 items-center rounded-full border border-gray-200 px-2.5 font-medium text-gray-700 text-xs shadow-sm transition-all hover:bg-gray-100"
			classList={{
				"bg-gray-100 ring-1 ring-gray-300": isActive(),
				"bg-white": !isActive(),
			}}
			onClick={toggleInstance}
		>
			All
		</button>
	);
};

const Instance = (props: { instance: string }) => {
	const color = () => useInstanceColor(props.instance);
	const { isActive, toggleInstance } = useInstanceIsActive(props.instance);

	return (
		<button
			type="button"
			class="flex h-7 items-center gap-1.5 rounded-full border border-gray-200 px-2.5 font-medium text-gray-700 text-xs shadow-sm transition-all hover:bg-gray-100"
			classList={{
				"bg-gray-100 ring-1 ring-gray-300": isActive(),
				"bg-white": !isActive(),
			}}
			onClick={toggleInstance}
		>
			<span
				class="inline-block h-2.5 w-2.5 rounded-full"
				style={{ "background-color": `rgb(${color()})` }}
			/>
			{props.instance}
		</button>
	);
};

const mergeInstancesMap = (instancesMap: Record<string, boolean>): string[] => {
	return Object.keys(instancesMap).filter((instance) => instancesMap[instance]);
};

const useInstanceIsActive = (instance: string | "all") => {
	const searchParams = Route.useSearch();
	const navigate = Route.useNavigate();

	const allActive = createMemo(
		on(
			() => searchParams().search,
			() => {
				const instanceSelected =
					Object.keys(mapInstances(searchParams().instances)).length > 0;
				return !instanceSelected;
			},
		),
	);

	const isActive = () => {
		if (instance === "all") {
			return allActive();
		}

		const instanceSearchParams = mapInstances(searchParams().instances);
		return instanceSearchParams[instance] ?? false;
	};

	const addInstance = () => {
		if (instance === "all") {
			navigate({
				search: { instances: undefined },
			});
			return;
		}

		const instanceSearchParams = mapInstances(searchParams().instances);
		instanceSearchParams[instance] = true;
		navigate({
			search: { instances: mergeInstancesMap(instanceSearchParams) },
		});
	};

	const removeInstance = () => {
		if (instance === "all") {
			console.warn("Cannot remove all instances");
			return;
		}

		const instanceSearchParams = mapInstances(searchParams().instances);
		instanceSearchParams[instance] = false;
		const newInstances = mergeInstancesMap(instanceSearchParams);
		batch(() => {
			navigate({
				search: { instances: newInstances },
			});
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

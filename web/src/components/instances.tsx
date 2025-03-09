import { For, Suspense, batch, createMemo, on } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import type { WsActions } from "@/lib/store/connection";
import { useInstances } from "@/lib/hooks/logs/use-instances";
import { Route } from "@/routes/logs.$clientId.index";

export const Instances = (props: { clientId: string }) => {
	const instances = useInstances(() => props.clientId);

	return (
		<nav class="gap-2 border border-sky-700 p-2">
			<p>Instances</p>
			<Suspense fallback={<div>Loading...</div>}>
				<div class="scrollbar-thin scrollbar-track-white scrollbar-thumb-zinc-700 flex gap-2 overflow-x-scroll p-2">
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

const Instance = (props: { instance: string }) => {
	const color = () => useInstanceColor(props.instance);
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
				<circle cx="50" cy="50" r="25" fill={`rgb(${color()})`} />
			</svg>

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

	//const [searchParams, setSearchParams] = useSearchParams<{
	//	instance: string | string[];
	//}>();

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

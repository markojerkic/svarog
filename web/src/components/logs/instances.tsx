import { For, Suspense, batch, createMemo, on } from "solid-js";
import { useInstanceColor } from "@/lib/hooks/instance-color";
import type { WsActions } from "@/lib/store/connection";
import { useInstances } from "@/lib/hooks/logs/use-instances";
import { Route } from "@/routes/__authenticated/logs.$clientId.index";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
	DropdownMenuCheckboxItem,
} from "@/components/ui/dropdown-menu";

export const Instances = (props: { clientId: string }) => {
	const instances = useInstances(() => props.clientId);

	return (
		<DropdownMenu>
			<DropdownMenuTrigger class="flex items-center gap-2 rounded-md border border-gray-200 px-3 py-1.5 font-medium text-gray-700 text-sm shadow-sm hover:bg-gray-50">
				Instances
			</DropdownMenuTrigger>
			<DropdownMenuContent class="min-w-[180px] p-1">
				<Suspense
					fallback={<DropdownMenuItem disabled>Loading...</DropdownMenuItem>}
				>
					<InstanceCheckboxItem instance="all" />
					<For each={instances.data}>
						{(instance) => <InstanceCheckboxItem instance={instance} />}
					</For>
				</Suspense>
			</DropdownMenuContent>
		</DropdownMenu>
	);
};

export const NOOP_WS_ACTIONS: WsActions = {
	setInstances: () => {},
	addSubscription: () => {},
	removeSubscription: () => {},
	close: () => {},
};

const InstanceCheckboxItem = (props: { instance: string | "all" }) => {
	const { isActive, toggleInstance } = useInstanceIsActive(props.instance);
	const color = () =>
		props.instance !== "all" ? useInstanceColor(props.instance) : null;

	return (
		<DropdownMenuCheckboxItem
			checked={isActive()}
			onChange={toggleInstance}
			class="flex items-center gap-2"
		>
			{props.instance !== "all" && (
				<span
					class="inline-block h-2.5 w-2.5 flex-shrink-0 rounded-full"
					style={{
						"background-color": color() ? `rgb(${color()})` : undefined,
					}}
				/>
			)}
			<span class="text-sm">
				{props.instance === "all" ? "All" : props.instance}
			</span>
		</DropdownMenuCheckboxItem>
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

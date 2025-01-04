import Resizable from "@corvu/resizable";
import { cookieStorage, makePersisted } from "@solid-primitives/storage";
import { createSignal, type ParentProps } from "solid-js";
import { ResizableHandle, ResizablePanel } from "../ui/resizable";
import { cn } from "@/lib/cn";
import { Gauge, Settings, User } from "lucide-solid/icons";
import { NavListItems } from "./admin-nav";

export const AdminLayout = (props: ParentProps) => {
	const [sizes, setSizes] = makePersisted(createSignal<number[]>([]), {
		name: "admin-panel-resizable",
		storage: cookieStorage,
		storageOptions: {
			path: "/",
		},
	});
	const [collapsed, setCollapsed] = createSignal(false);

	return (
		<Resizable sizes={sizes()} onSizesChange={setSizes}>
			<ResizablePanel
				initialSize={sizes()[0] ?? 0.2}
				minSize={0.1}
				maxSize={0.2}
				collapsible
				onCollapse={(e) => setCollapsed(e === 0)}
				onExpand={() => setCollapsed(false)}
				class={cn(
					collapsed() && "min-w-[50px] transition-all duration-300 ease-in-out",
				)}
			>
				<NavListItems
					isCollapsed={collapsed()}
					links={[
						{
							title: "Dashboard",
							icon: <Gauge />,
							variant: "default",
						},
						{
							title: "Users",
							icon: <User />,
							variant: "default",
						},
						{
							title: "Settings",
							icon: <Settings />,
							variant: "default",
						},
					]}
				/>
			</ResizablePanel>
			<ResizableHandle withHandle />
			<ResizablePanel
				initialSize={sizes()[1] ?? 0.8}
				minSize={0.8}
				maxSize={0.9}
			>
				Ovo je ostatak
				{props.children}
			</ResizablePanel>
		</Resizable>
	);
};

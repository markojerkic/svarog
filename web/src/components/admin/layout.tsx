import { NavListItems } from "@/components/admin/admin-nav";
import { ResizableHandle, ResizablePanel } from "@/components/ui/resizable";
import { cn } from "@/lib/cn";
import Resizable from "@corvu/resizable";
import { cookieStorage, makePersisted } from "@solid-primitives/storage";
import Gauge from "lucide-solid/icons/gauge";
import User from "lucide-solid/icons/user";
import Settings from "lucide-solid/icons/settings";
import { type ParentProps, createSignal } from "solid-js";

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
							to: "/admin/dashboard",
							icon: <Gauge />,
						},
						{
							title: "Projects",
							to: "/admin/projects",
							icon: <Settings />,
						},
						{
							title: "Users",
							to: "/admin/users",
							icon: <User />,
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
				{props.children}
			</ResizablePanel>
		</Resizable>
	);
};

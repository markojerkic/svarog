import Resizable from "@corvu/resizable";
import { cookieStorage, makePersisted } from "@solid-primitives/storage";
import { createSignal, type ParentProps } from "solid-js";
import { ResizableHandle, ResizablePanel } from "@/components/ui/resizable";
import { cn } from "@/lib/cn";
import Gauge from "lucide-solid/icons/gauge";
import Settings from "lucide-solid/icons/settings";
import User from "lucide-solid/icons/user";
import { NavListItems } from "@/components/admin/admin-nav";

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
							href: "/admin",
							icon: <Gauge />,
						},
						{
							title: "Users",
							href: "/admin/users",
							icon: <User />,
						},
						{
							title: "Projects",
							href: "/admin/projects",
							icon: <Settings />,
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

import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import type { DialogTriggerProps } from "@kobalte/core/dialog";
import { createForm, valiForm } from "@modular-forms/solid";
import { TextFormField } from "@/components/ui/textfield";
import { createSignal, Show } from "solid-js";
import { toast } from "solid-sonner";
import Plus from "lucide-solid/icons/plus";
import {
	DropdownMenuItem,
	DropdownMenuItemLabel,
} from "@/components/ui/dropdown-menu";
import {
	type AddClientInput,
	addClientSchema,
	useAddClient,
} from "@/lib/hooks/projects/add-remove-client";

export const AddClient = (props: { projectId: string }) => {
	const [open, setOpen] = createSignal(false);

	return (
		<Dialog open={open()} onOpenChange={setOpen}>
			<DialogTrigger
				as={(props: DialogTriggerProps) => (
					<DropdownMenuItem {...props} closeOnSelect={false}>
						<Plus class="h-5" />
						<DropdownMenuItemLabel>Add client</DropdownMenuItemLabel>
					</DropdownMenuItem>
				)}
			/>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Add a client to the project</DialogTitle>
					<DialogDescription>
						Enter the name of the new client
					</DialogDescription>
				</DialogHeader>
				<AddClientForm
					projectId={props.projectId}
					onSuccess={() => setOpen(false)}
				/>
			</DialogContent>
		</Dialog>
	);
};

const AddClientForm = (props: { projectId: string; onSuccess: () => void }) => {
	const [form, { Form, Field }] = createForm<AddClientInput>({
		validate: valiForm(addClientSchema),
		initialValues: { projectId: props.projectId },
	});
	const createClient = useAddClient(form);

	const handleSubmit = async (values: AddClientInput) => {
		createClient.mutateAsync(values).then(() => {
			props.onSuccess();
			toast.success("Client created");
		});
	};

	return (
		<Form class="flex flex-col gap-4" onSubmit={handleSubmit}>
			<Field type="string" name="projectId">
				{(field, props) => (
					<input type="hidden" {...props} value={field.value} />
				)}
			</Field>

			<Field type="string" name="clientName">
				{(field, props) => (
					<TextFormField
						{...props}
						type="text"
						label="Client name"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>

			<Show when={createClient.isError}>
				<p class="py-2 text-red-500">{createClient.error?.message}</p>
			</Show>

			<Button
				type="submit"
				disabled={createClient.isPending || form.submitting}
			>
				Submit
			</Button>
		</Form>
	);
};

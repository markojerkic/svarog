import type { DialogTriggerProps } from "@kobalte/core/dialog";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogCloseButton,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { TextFormField } from "@/components/ui/textfield";
import { createForm, setError, valiForm } from "@modular-forms/solid";
import { createSignal, Match, Show, Switch } from "solid-js";
import {
	type RegisterInput,
	registerSchema,
	useRegister,
} from "@/lib/hooks/auth/register";
import { toast } from "solid-sonner";

export const NewUserDialog = () => {
	const [open, setOpen] = createSignal(false);
	const [loginToken, setLoginToken] = createSignal<string>();

	return (
		<Dialog open={open()} onOpenChange={setOpen}>
			<DialogTrigger
				as={(props: DialogTriggerProps) => (
					<Button variant="outline" {...props}>
						New user
					</Button>
				)}
			/>
			<DialogContent class="sm:max-w-[425px]">
				<Switch>
					<Match when={loginToken()} keyed>
						{(token) => <CopyLoginTokenButton loginToken={token} />}
					</Match>
					<Match when={!loginToken()}>
						<DialogHeader>
							<DialogTitle>Create new user</DialogTitle>
							<DialogDescription>
								Enter the user's information to create a new user.
							</DialogDescription>
						</DialogHeader>
						<div class="grid gap-4 py-4">
							<RegisterForm onSuccess={setLoginToken} />
						</div>
						<DialogFooter>
							<Button type="submit">Save changes</Button>
						</DialogFooter>
					</Match>
				</Switch>
			</DialogContent>
		</Dialog>
	);
};

const RegisterForm = (props: { onSuccess: (loginToken: string) => void }) => {
	const [form, { Form, Field }] = createForm<RegisterInput>({
		validate: valiForm(registerSchema),
	});
	const register = useRegister(form);

	const handleSubmit = (values: RegisterInput) => {
		if (values.password !== values.repeatedPassword) {
			setError(form, "password", "Passwords do not match");
			setError(form, "repeatedPassword", "Passwords do not match");
			return;
		}

		register.mutate(values, {
			onSuccess: (token) => {
				props.onSuccess(token);
			},
		});
	};

	return (
		<Form onSubmit={handleSubmit}>
			<Field type="string" name="username">
				{(field, props) => (
					<TextFormField
						{...props}
						type="text"
						label="Username"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>
			<Field type="string" name="firstName">
				{(field, props) => (
					<TextFormField
						{...props}
						type="text"
						label="First name"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>
			<Field type="string" name="lastName">
				{(field, props) => (
					<TextFormField
						{...props}
						type="text"
						label="Last name"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>

			<Field type="string" name="password">
				{(field, props) => (
					<TextFormField
						{...props}
						type="password"
						label="Password"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>

			<Field type="string" name="repeatedPassword">
				{(field, props) => (
					<TextFormField
						{...props}
						type="password"
						label="Repeated password"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>

			<Show when={register.isError}>
				<p class="py-2 text-red-500">{register.error?.message}</p>
			</Show>

			<Button type="submit" disabled={register.isPending || form.submitting}>
				Register
			</Button>
		</Form>
	);
};

const CopyLoginTokenButton = (props: { loginToken: string }) => {
	const copy = () => {
		const currentDomain = window.location.origin;
		const loginUrl = `${currentDomain}/auth/login/${props.loginToken}`;
		navigator.clipboard.writeText(loginUrl);
		toast.success("Login token copied to clipboard");
	};

	return (
		<>
			<DialogHeader>
				<DialogTitle>Login token</DialogTitle>
				<DialogDescription>
					Copy the login token below to share with the user.
				</DialogDescription>
			</DialogHeader>
			<Button onClick={copy}>Copy login token</Button>
			<DialogFooter>
				<DialogCloseButton>Close</DialogCloseButton>
			</DialogFooter>
		</>
	);
};

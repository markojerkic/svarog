import { createFileRoute, } from "@tanstack/solid-router";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { TextFormField } from "@/components/ui/textfield";
import { type LoginInput, loginSchema, useLogin } from "@/lib/hooks/auth/login";
import { createForm, valiForm } from "@modular-forms/solid";
import { Show } from "solid-js";
import * as v from "valibot";

const schema = v.object({
	redirect: v.optional(v.pipe(v.string())),
	redirectSearch: v.optional(v.any()),
});

export const Route = createFileRoute("/auth/login")({
	component: RouteComponent,
	validateSearch: schema,
});

function RouteComponent() {
	return (
		<Card class="container my-16 w-full md:w-[70%] lg:w-[50%]">
			<CardHeader>
				<CardTitle>Login</CardTitle>
			</CardHeader>
			<CardContent>
				<div class="grid gap-2">
					<LoginForm />
				</div>
			</CardContent>
		</Card>
	);
}

const LoginForm = () => {
	const navigate = Route.useNavigate();
	const searchParams = Route.useSearch();

	const [form, { Form, Field }] = createForm<LoginInput>({
		validate: valiForm(loginSchema),
	});
	const login = useLogin(form);

	const handleSubmit = (values: LoginInput) => {
		login.mutate(values, {
			onSuccess: () => {
				if (searchParams().redirect) {
					navigate({
						to: searchParams().redirect,
						search: searchParams().redirectSearch,
					});
					return;
				}
				navigate({
					to: "/",
				});
			},
		});
	};

	return (
		<Form class="flex flex-col gap-2" onSubmit={handleSubmit}>
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

			<Show when={login.isError}>
				<p class="py-2 text-red-500">{login.error?.message}</p>
			</Show>

			<Button type="submit" disabled={login.isPending || form.submitting}>
				Login
			</Button>
		</Form>
	);
};

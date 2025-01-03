import {
	Card,
	CardContent,
	CardFooter,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import {
	TextField,
	TextFieldLabel,
	TextFieldRoot,
	TextFormField,
} from "@/components/ui/textfield";
import { createForm, valiForm } from "@modular-forms/solid";
import * as v from "valibot";

export default () => {
	return (
		<Card class="container w-full md:w-[70%] lg:w-[50%]">
			<CardHeader>
				<CardTitle>Login</CardTitle>
			</CardHeader>
			<CardContent>
				<div class="grid gap-2">
					<LoginForm />
				</div>
			</CardContent>
			<CardFooter>
				<p>Card Footer</p>
			</CardFooter>
		</Card>
	);
};

const loginSchema = v.object({
	email: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your email"),
		v.email("Please enter a valid email"),
	),
	password: v.pipe(
		v.string("Must be a string"),
		v.nonEmpty("Please enter your password"),
		v.minLength(6, "Password must be at least 6 characters"),
	),
});

const LoginForm = () => {
	const [_, { Form, Field }] = createForm<v.InferInput<typeof loginSchema>>({
		validate: valiForm(loginSchema),
	});

	return (
		<Form>
			<Field type="string" name="email">
				{(field, props) => (
					<TextFormField
						{...props}
						type="email"
						label="Email"
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
						type="email"
						label="Email"
						error={field.error}
						value={field.value as string | undefined}
						required
					/>
				)}
			</Field>
		</Form>
	);
};

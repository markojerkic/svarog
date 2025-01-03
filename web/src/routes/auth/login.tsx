import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import {
    TextFieldErrorMessage,
    TextFormField,
} from "@/components/ui/textfield";
import {
    type LoginInput,
    loginSchema,
    useLogin,
} from "@/lib/hooks/auth/login-register";
import { createForm, valiForm } from "@modular-forms/solid";
import { createEffect, Show } from "solid-js";

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

const LoginForm = () => {
    const [form, { Form, Field }] = createForm<LoginInput>({
        validate: valiForm(loginSchema),
    });

    const login = useLogin(form);

    const handleSubmit = async (values: LoginInput) => {
        login.action.mutate(values);
    };

    createEffect(() => {
        if (login.error()) {
            console.log("Error", login.error());
        }

        if (login.action.isError) {
            console.log("Error iz akcije", login.action.error);
        }
    });

    return (
        <>
            <Form onSubmit={handleSubmit}>
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
                            type="password"
                            label="Password"
                            error={field.error}
                            value={field.value as string | undefined}
                            required
                        />
                    )}
                </Field>

                <Button type="submit" disabled={login.action.isPending}>
                    Login
                </Button>
            </Form>
            <Show when={login.error()}>
                <p>Error: {login.error()?.message}</p>
                <TextFieldErrorMessage>{login.error()?.message}</TextFieldErrorMessage>
            </Show>
            <p class="bg-green-500 p-4">{login.error()?.message}</p>
        </>
    );
};

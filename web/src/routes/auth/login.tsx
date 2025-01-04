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
    api,
    type LoginInput,
    loginSchema,
    useLogin,
} from "@/lib/hooks/auth/login-register";
import { createForm, valiForm } from "@modular-forms/solid";
import { createMutation } from "@tanstack/solid-query";
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
    const login = createMutation(() => ({
        mutationKey: ["login"],
        mutationFn: async (input: LoginInput) => {
            return api.post("/v1/auth/login", input);
        },
    }));

    const handleSubmit = async (values: LoginInput) => {
        login.mutate(values);
        //try {
        //    await login.mutateAsync(values);
        //} catch (error) {
        //    console.error("form await mutate async", error);
        //}
    };

    return (
        <>
            <Show when={login.isPending}>
                <p>Loading...</p>
            </Show>
            <Show when={login.isSuccess}>
                <p>Success</p>
            </Show>
            <Show when={login.isError}>
                <p>Error: {login.error?.message}</p>
            </Show>
            <Show when={login.isIdle}>
                <p>Idle</p>
            </Show>

            <Form onSubmit={(props) => handleSubmit(props)}>
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

                <Button type="submit" disabled={login.isPending || form.submitting}>
                    Login
                </Button>
            </Form>
        </>
    );
};

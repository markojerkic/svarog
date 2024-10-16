import { renderHook, render } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import {
	type ParentProps,
	createSignal,
	createRoot,
	catchError,
	type Owner,
	Suspense,
	createEffect,
} from "solid-js";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { createLogQuery } from "~/lib/store/query";
import type { LogLine } from "~/lib/store/log-store";

const waitFor = (fn: () => boolean, owner?: Owner) => {
	let done: () => void;
	let fail: (error: unknown) => void;
	const promise = new Promise<void>((resolve, reject) => {
		done = resolve;
		fail = reject;
	});

	createRoot((dispose) => {
		catchError(async () => {
			let isDone = false;
			while (!isDone) {
				isDone = fn();
				await new Promise((resolve) => setTimeout(resolve, 1000));
			}
			done();
			dispose();
		}, fail);
	}, owner);
	return promise;
};

const mockData = [
	{
		id: "1",
		content: "Hello",
		timestamp: new Date().getTime(),
		sequenceNumber: 1,
		client: {
			clientId: "marko",
			ipAddress: "::1",
		},
	},
] satisfies LogLine[];

const mockData2 = [
	{
		id: "2",
		content: "Hello from data 2",
		timestamp: new Date().getTime(),
		sequenceNumber: 1,
		client: {
			clientId: "jerkic",
			ipAddress: "::2",
		},
	},
] satisfies LogLine[];

const handlers = [
	http.get(`${import.meta.env.VITE_API_URL}/v1/logs/marko`, () => {
		return HttpResponse.json(mockData);
	}),
	http.get(`${import.meta.env.VITE_API_URL}/v1/logs/jerkic`, () => {
		return HttpResponse.json(mockData2);
	}),
];
const mockServer = setupServer(...handlers);

describe("createLoqQuery", () => {
	beforeAll(() => mockServer.listen());
	afterEach(() => mockServer.resetHandlers());
	afterAll(() => mockServer.close());

	const queryClient = new QueryClient();

	const Wrapper = (props: ParentProps) => (
		<QueryClientProvider client={queryClient}>
			{props.children}
		</QueryClientProvider>
	);

	it("should initialize and return data", async () => {
		const [clientId, _setClientId] = createSignal("marko");
		const [selectedInstances, _setSelectedInstances] = createSignal<
			string[] | undefined
		>(undefined);
		const [search, _setSearch] = createSignal<string | undefined>(undefined);
		const { result } = renderHook(
			() => createLogQuery(clientId, selectedInstances, search),
			{
				wrapper: Wrapper,
			},
		);
		await result.queryDetails.fetchNextPage();

		await waitFor(() => {
			return result.queryDetails.isSuccess;
		});
		expect(
			result.queryDetails.isSuccess,
			"Query needs to resolve to success",
		).toBeTruthy();
		expect(result.data.size, "Data should hold exactly one element").toEqual(1);
		expect(result.data.get(0), "Query data needs to be as expected").toEqual(
			mockData[0],
		);
	});

	it("should refetch data when search changes", async () => {
		const [clientId, setClientId] = createSignal("marko");
		const [selectedInstances, _setSelectedInstances] = createSignal<
			string[] | undefined
		>(undefined);
		const [search, _setSearch] = createSignal<string | undefined>(undefined);
		const { result } = renderHook(
			() => createLogQuery(clientId, selectedInstances, search),
			{
				wrapper: Wrapper,
			},
		);
		await result.queryDetails.fetchNextPage();

		await waitFor(() => {
			return result.queryDetails.isSuccess;
		});
		expect(
			result.queryDetails.isSuccess,
			"Query needs to resolve to success",
		).toBeTruthy();
		expect(result.data.size, "Data should hold exactly one element").toEqual(1);
		expect(result.data.get(0), "Query data needs to be as expected").toEqual(
			mockData[0],
		);

		// Change clientId to jerkic

		setClientId("jerkic");
		await waitFor(() => {
			const isSuccess = result.queryDetails.isSuccess;
			return isSuccess;
		});

		expect(result.data.size, "Data should hold exactly one element").toEqual(1);
		expect(result.data.get(0), "Query data needs to be as expected").toEqual(
			mockData2[0],
		);
	});

	it("should add value to DOM after insert", async () => {
		const TestBed = () => {
			const [clientId, _setClientId] = createSignal("marko");
			const [selectedInstances, _setSelectedInstances] = createSignal<
				string[] | undefined
			>(undefined);
			const [search, _setSearch] = createSignal<string | undefined>(undefined);

			const query = createLogQuery(clientId, selectedInstances, search);

			return (
				<Suspense>
					<div id="target">{query.data.get(0)?.content}</div>
				</Suspense>
			);
		};

		const { findByText } = render(() => (
			<Wrapper>
				<TestBed />
			</Wrapper>
		));

		expect(await findByText("Hello")).toBeTruthy();
	});

	it("should add value to DOM after insert", async () => {
		const TestBed = () => {
			const [clientId, setClientId] = createSignal("marko");
			const [selectedInstances, _setSelectedInstances] = createSignal<
				string[] | undefined
			>(undefined);
			const [search, _setSearch] = createSignal<string | undefined>(undefined);

			const query = createLogQuery(clientId, selectedInstances, search);

			createEffect(() => {
				setClientId("jerkic");
			});

			return (
				<Suspense>
					<div id="target">{query.data.get(0)?.content}</div>
				</Suspense>
			);
		};

		const { findByText } = render(() => (
			<Wrapper>
				<TestBed />
			</Wrapper>
		));

		expect(await findByText("Hello from data 2")).toBeTruthy();
	});
});

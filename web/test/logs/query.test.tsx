import { renderHook, waitFor } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { type ParentProps, createSignal } from "solid-js";
import { afterAll, afterEach, beforeAll, describe, expect, test } from "vitest";
import { createLoqQuery } from "~/lib/store/query";
import type { LogLine } from "~/lib/store/log-store";

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

const handlers = [
	http.get(`${import.meta.env.VITE_API_URL}/v1/logs/*`, () => {
		console.log("Hello from mock");
		return HttpResponse.json(mockData);
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

	const [clientId, _setClientId] = createSignal("marko");
	const [selectedInstances, _setSelectedInstances] = createSignal<
		string[] | undefined
	>(undefined);
	const [search, _setSearch] = createSignal<string | undefined>(undefined);

	test("should initialize and return data", async () => {
		const { result } = renderHook(
			() => createLoqQuery(clientId, selectedInstances, search),
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
});

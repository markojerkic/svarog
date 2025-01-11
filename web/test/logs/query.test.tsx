import { renderHook } from "@solidjs/testing-library";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { Component, ParentProps } from "solid-js";
import {
	afterAll,
	afterEach,
	beforeAll,
	describe,
	expect,
	it,
	vi,
} from "vitest";
import { useLogStore, type LogLine } from "@/lib/hooks/use-log-store";

const mockData: LogLine[] = [
	{
		id: "1",
		content: "First log entry",
		timestamp: new Date().getTime(),
		sequenceNumber: 1,
		client: {
			clientId: "client1",
			ipAddress: "::1",
		},
	},
	{
		id: "2",
		content: "Second log entry",
		timestamp: new Date().getTime() + 1000,
		sequenceNumber: 2,
		client: {
			clientId: "client1",
			ipAddress: "::1",
		},
	},
];

const mockDataPage2: LogLine[] = [
	{
		id: "3",
		content: "Earlier log entry",
		timestamp: new Date().getTime() - 1000,
		sequenceNumber: 0,
		client: {
			clientId: "client1",
			ipAddress: "::1",
		},
	},
];

const handlers = [
	http.get(`${import.meta.env.VITE_API_URL}/v1/logs/client1`, () => {
		return HttpResponse.json(mockData);
	}),
	// Handler for previous page fetch with cursor
	http.get(`${import.meta.env.VITE_API_URL}/v1/logs/client1`, ({ request }) => {
		const url = new URL(request.url);
		const direction = url.searchParams.get("direction");
		if (direction === "backward") {
			return HttpResponse.json(mockDataPage2);
		}
		return HttpResponse.json(mockData);
	}),
	// Handler for search
	http.get(`${import.meta.env.VITE_API_URL}/v1/logs/client1/search`, () => {
		return HttpResponse.json([mockData[0]]);
	}),
];

const mockServer = setupServer(...handlers);

vi.mock("@/lib/hooks/use-scroll-event", () => ({
	useScrollEvent: () => ({
		scrollToBottom: vi.fn(),
		scrollToIndex: vi.fn(),
	}),
}));

describe("useLogStore", () => {
	beforeAll(() => mockServer.listen());
	afterEach(() => {
		mockServer.resetHandlers();
		vi.clearAllMocks();
	});
	afterAll(() => mockServer.close());

	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				retry: false,
			},
		},
	});

	const Wrapper: Component<ParentProps> = (props) => (
		<QueryClientProvider client={queryClient}>
			{props.children}
		</QueryClientProvider>
	);

	it("should initialize and fetch initial logs", async () => {
		const { result } = renderHook(
			() =>
				useLogStore(() => ({
					clientId: "client1",
					selectedInstances: [],
				})),
			{ wrapper: Wrapper },
		);

		// Wait for initial fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(2);
		});

		expect(result.logs.get(0)).toEqual(expect.objectContaining({ id: "1" }));
		expect(result.logs.get(1)).toEqual(expect.objectContaining({ id: "2" }));
	});

	it("should fetch previous page when requested", async () => {
		const { result } = renderHook(
			() =>
				useLogStore(() => ({
					clientId: "client1",
					selectedInstances: [],
				})),
			{ wrapper: Wrapper },
		);

		// Wait for initial fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(2);
		});

		// Request previous page
		expect(result.state.type).toBe("idle");
		if (result.state.type === "idle") {
			result.state.value.fetchPreviousPage();
		}

		// Wait for fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(3);
		});

		expect(result.logs.get(0)).toEqual(expect.objectContaining({ id: "1" }));
		expect(result.logs.get(1)).toEqual(expect.objectContaining({ id: "2" }));
		expect(result.logs.get(2)).toEqual(expect.objectContaining({ id: "3" }));
	});

	it("should reset store when props change", async () => {
		let searchQuery: string | undefined = undefined;
		const { result } = renderHook(
			() =>
				useLogStore(() => ({
					clientId: "client1",
					selectedInstances: [],
					searchQuery,
				})),
			{ wrapper: Wrapper },
		);

		// Wait for initial fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(2);
		});

		// Change search query
		searchQuery = "test";

		// Wait for new fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(1);
		});

		expect(result.logs.get(0)).toEqual(expect.objectContaining({ id: "1" }));
	});

	it("should maintain sorted order when inserting logs", async () => {
		const { result } = renderHook(
			() =>
				useLogStore(() => ({
					clientId: "client1",
					selectedInstances: [],
				})),
			{ wrapper: Wrapper },
		);

		// Wait for initial fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(2);
		});

		// Request previous page
		expect(result.state.type).toBe("idle");
		if (result.state.type === "idle") {
			result.state.value.fetchPreviousPage();
		}

		// Wait for fetch to complete
		await vi.waitFor(() => {
			expect(result.logs.size).toBe(3);
		});

		const logs = result.logs;
		expect(logs.size).toEqual(3);
		expect(logs.get(0)?.sequenceNumber).toBeLessThan(
			logs.get(1)?.sequenceNumber!,
		);
		expect(logs.get(1)?.sequenceNumber).toBeLessThan(
			logs.get(2)?.sequenceNumber!,
		);
	});
});

import { expect, describe, it } from "vitest";
import { SortedList } from "~/lib/store/sorted-list";

describe("Sorted list", () => {
	it("Sort on insert and return correct item at index", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.get(0)).toBe(1);
		expect(list.get(1)).toBe(2);
		expect(list.get(2)).toBe(3);
		expect(list.get(3)).toBe(4);
		expect(list.get(4)).toBe(5);
	});

	it("Reverse sort on insert and return correct item at index", () => {
		const list = new SortedList<number>((a, b) => b - a);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.get(0)).toBe(5);
		expect(list.get(1)).toBe(4);
		expect(list.get(2)).toBe(3);
		expect(list.get(3)).toBe(2);
		expect(list.get(4)).toBe(1);
	});

	it("Return correct size", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.size).toBe(5);
	});
});

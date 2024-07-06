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

	it("Has correct head and tail", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.getHead()?.value).toBe(1);
		expect(list.getHead()?.value).toBe(5);
	});

	it("Has correct head and tail with reverse sort", () => {
		const list = new SortedList<number>((a, b) => b - a);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.getHead()?.value).toBe(5);
		expect(list.getHead()?.value).toBe(1);
	});

	it("Has correct head and tail with multiple input lines", () => {
		const list = new SortedList<number>((a, b) => b - a);
		list.insert(5);

		list.insertMany([1, 3, 2, 4]);

		expect(list.getHead()?.value).toBe(5);
		expect(list.getHead()?.value).toBe(1);
	});

	it("Has correct items after insert many", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		list.insertMany([6, 7, 8]);

		expect(list.get(0)).toBe(1);
		expect(list.get(1)).toBe(2);
		expect(list.get(2)).toBe(3);
		expect(list.get(3)).toBe(4);
		expect(list.get(4)).toBe(5);
		expect(list.get(5)).toBe(6);
		expect(list.get(6)).toBe(7);
		expect(list.get(7)).toBe(8);
	});

	it("Has correct items after prepended many", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(10);
		list.insert(11);
		list.insert(12);
		list.insert(13);
		list.insert(14);

		list.insertMany([6, 7, 8]);

		expect(list.get(0)).toBe(6);
		expect(list.get(1)).toBe(7);
		expect(list.get(2)).toBe(8);
		expect(list.get(3)).toBe(10);
		expect(list.get(4)).toBe(11);
		expect(list.get(5)).toBe(12);
		expect(list.get(6)).toBe(13);
		expect(list.get(7)).toBe(14);
	});

	it("Head and tail are equal if only one element", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);

		expect(list.getHead()).toBe(list.getHead());
	});

	it("Head and tail null if empty list", () => {
		const list = new SortedList<number>((a, b) => a - b);

		expect(list.size).toBe(0);
		expect(list.getHead()).toBeNull();
		expect(list.getHead()).toBeNull();
	});
});
